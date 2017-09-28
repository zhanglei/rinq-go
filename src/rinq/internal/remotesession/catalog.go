package remotesession

import (
	"context"
	"sync"

	"github.com/rinq/rinq-go/src/rinq"
	"github.com/rinq/rinq-go/src/rinq/ident"
	revisionpkg "github.com/rinq/rinq-go/src/rinq/internal/revision"
	"github.com/rinq/rinq-go/src/rinq/internal/syncutil"
)

type catalog struct {
	id     ident.SessionID
	client *client

	mutex      sync.RWMutex
	highestRev ident.Revision
	cache      attrTableCache
	isClosed   bool
}

func newCatalog(id ident.SessionID, client *client) *catalog {
	return &catalog{
		id:     id,
		client: client,

		cache: attrTableCache{},
	}
}

func (c *catalog) Head(ctx context.Context) (rinq.Revision, error) {
	unlock := syncutil.RLock(&c.mutex)
	defer unlock()

	if c.isClosed {
		return nil, rinq.NotFoundError{ID: c.id}
	}

	unlock()

	rev, _, err := c.client.Fetch(ctx, c.id, "", nil)

	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.updateState(rev, err)

	if err != nil {
		return nil, err
	}

	return &revision{
		ref:     c.id.At(c.highestRev),
		catalog: c,
	}, nil
}

func (c *catalog) At(rev ident.Revision) rinq.Revision {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	ref := c.id.At(rev)

	if c.isClosed {
		return revisionpkg.Closed(ref)
	}

	c.updateState(rev, nil)

	return &revision{
		ref:     ref,
		catalog: c,
	}
}

func (c *catalog) Fetch(
	ctx context.Context,
	rev ident.Revision,
	ns string,
	keys ...string,
) ([]rinq.Attr, error) {
	solvedAttrs, unsolvedKeys, err := c.fetchLocal(rev, ns, keys)
	if err != nil {
		return nil, err
	} else if len(unsolvedKeys) == 0 {
		return solvedAttrs, nil
	}

	fetchedRev, fetchedAttrs, err := c.client.Fetch(ctx, c.id, ns, unsolvedKeys)

	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.updateState(fetchedRev, err)

	if err != nil {
		return nil, err
	}

	if len(fetchedAttrs) == 0 {
		return solvedAttrs, nil
	}

	isStaleFetch := false
	cache, isExistingNamespace := c.cache[ns]

	for _, attr := range fetchedAttrs {
		entry := cache[attr.Key]

		// Update the cache entry if the fetched revision is newer.
		if fetchedRev > entry.FetchedAt {
			if cache == nil {
				cache = attrNamespaceCache{}
			}

			cache[attr.Key] = cachedAttr{attr, fetchedRev}
		}

		if isStaleFetch {
			continue
		}

		// The attribute hadn't been created at this revision, so we know it
		// had an empty value.
		if attr.CreatedAt > rev {
			continue
		}

		// The attribute has been changed since this revision, so we know it's
		// stale, but we continue through the loop to cache any other attributes.
		if attr.UpdatedAt > rev {
			isStaleFetch = true
			continue
		}

		// We just fetch the attribute, so we know it's valid right now.
		solvedAttrs = append(solvedAttrs, attr.Attr)
	}

	if !isExistingNamespace && cache != nil {
		c.cache[ns] = cache
	}

	if isStaleFetch {
		return nil, rinq.StaleFetchError{Ref: c.id.At(rev)}
	}

	return solvedAttrs, nil
}

func (c *catalog) TryUpdate(
	ctx context.Context,
	rev ident.Revision,
	ns string,
	attrs []rinq.Attr,
) (rinq.Revision, error) {
	unlock := syncutil.RLock(&c.mutex)
	defer unlock()

	if c.isClosed {
		return nil, rinq.NotFoundError{ID: c.id}
	}

	ref := c.id.At(rev)

	if c.highestRev > rev {
		return nil, rinq.StaleUpdateError{Ref: ref}
	}

	updateAttrs := make([]rinq.Attr, 0, len(attrs))

	// grab the cache for this namespace, note that this variable is used after
	// below when the write lock is acquired, as it will always point to the
	// same map value
	cache := c.cache[ns]

	for _, attr := range attrs {
		if entry, ok := cache[attr.Key]; ok {
			if entry.Attr.IsFrozen {
				if attr == entry.Attr.Attr {
					continue
				}

				return nil, rinq.FrozenAttributesError{Ref: ref}
			}

			if entry.FetchedAt == rev && attr == entry.Attr.Attr {
				continue
			}
		}

		updateAttrs = append(updateAttrs, attr)
	}

	unlock()

	updatedRev, returnedAttrs, err := c.client.Update(ctx, ref, ns, updateAttrs)

	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.updateState(updatedRev, err)

	if err != nil {
		return nil, err
	}

	// note that "cache" refers to the same map as when the read lock was
	// acquired above
	for _, attr := range returnedAttrs {
		entry := cache[attr.Key]
		if updatedRev > entry.FetchedAt {
			cache[attr.Key] = cachedAttr{attr, updatedRev}
		}
	}

	return &revision{
		ref:     c.id.At(c.highestRev),
		catalog: c,
	}, nil
}

func (c *catalog) TryDestroy(
	ctx context.Context,
	rev ident.Revision,
) error {
	unlock := syncutil.RLock(&c.mutex)
	defer unlock()

	if c.isClosed {
		return rinq.NotFoundError{ID: c.id}
	}

	ref := c.id.At(rev)

	if c.highestRev > rev {
		return rinq.StaleUpdateError{Ref: ref}
	}

	unlock()

	err := c.client.Close(ctx, ref)
	if err != nil {
		return err
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.isClosed = true

	return nil
}

func (c *catalog) fetchLocal(
	rev ident.Revision,
	ns string,
	keys []string,
) (
	solved []rinq.Attr,
	unsolved []string,
	err error,
) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	count := len(keys)
	solved = make([]rinq.Attr, 0, count)
	unsolved = make([]string, 0, count)

	cache := c.cache[ns]

	for _, key := range keys {
		if entry, ok := cache[key]; ok {
			// The attribute hadn't been created at this revision, so we know it
			// had an empty value.
			if entry.Attr.CreatedAt > rev {
				continue
			}

			// The attribute has been changed since this revision, so we can't
			// even fetch if from the remote peer.
			if entry.Attr.UpdatedAt > rev {
				err = rinq.StaleFetchError{Ref: c.id.At(rev)}
				return
			}

			// The attribute has been frozen, so it can't have changed, or we
			// already know the cache data is valid at or after the requested
			// revision.
			if entry.Attr.IsFrozen || rev <= entry.FetchedAt {
				solved = append(solved, entry.Attr.Attr)
				continue
			}
		}

		unsolved = append(unsolved, key)
	}

	if len(unsolved) > 0 && c.isClosed {
		err = rinq.NotFoundError{ID: c.id}
	}

	return
}

func (c *catalog) updateState(rev ident.Revision, err error) {
	if err != nil {
		if rinq.IsNotFound(err) {
			c.isClosed = true
		}
	} else if rev > c.highestRev {
		c.highestRev = rev
	}
}
