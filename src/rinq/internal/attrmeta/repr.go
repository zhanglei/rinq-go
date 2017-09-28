package attrmeta

import (
	"bytes"
	"io"

	"github.com/rinq/rinq-go/src/rinq/internal/bufferpool"
)

// WriteDiff writes a "diff" representation of attr to buffer.
func WriteDiff(buffer *bytes.Buffer, attr Attr) {
	if attr.Value == "" {
		Write(buffer, attr)
		return
	}

	if attr.CreatedAt == attr.UpdatedAt {
		buffer.WriteString("+")
	}

	buffer.WriteString(attr.Key)
	if attr.IsFrozen {
		buffer.WriteString("@")
	} else {
		buffer.WriteString("=")
	}
	buffer.WriteString(attr.Value)
}

// WriteDiffSlice writes a "diff" representation of attrs to buffer.
func WriteDiffSlice(buffer *bytes.Buffer, attrs []Attr) {
	for index, attr := range attrs {
		if index != 0 {
			buffer.WriteString(", ")
		}

		WriteDiff(buffer, attr)
	}
}

// Write writes a representation of attr to the buffer.
func Write(buffer *bytes.Buffer, attr Attr) {
	if attr.Value == "" {
		if attr.IsFrozen {
			buffer.WriteString("!")
		} else {
			buffer.WriteString("-")
		}
		buffer.WriteString(attr.Key)
	} else {
		buffer.WriteString(attr.Key)
		if attr.IsFrozen {
			buffer.WriteString("@")
		} else {
			buffer.WriteString("=")
		}
		buffer.WriteString(attr.Value)
	}
}

// WriteSlice writes a representation of attrs to the buffer.
func WriteSlice(buffer *bytes.Buffer, attrs []Attr) {
	for index, attr := range attrs {
		if index != 0 {
			buffer.WriteString(", ")
		}

		Write(buffer, attr)
	}
}

// WriteTable writes a respresentation of attrs to the buffer.
// Non-frozen attributes with empty-values are omitted.
func WriteTable(buffer *bytes.Buffer, attrs Table) {
	for _, attr := range attrs {
		if !attr.IsFrozen && attr.Value == "" {
			continue
		}

		if buffer.Len() != 0 {
			buffer.WriteString(", ")
		}

		Write(buffer, attr)
	}
}

// WriteNamespacedTable writes a respresentation of attrs to the buffer.
// Non-frozen attributes with empty-values are omitted.
func WriteNamespacedTable(buffer *bytes.Buffer, attrs NamespacedTable) {
	// TODO: review output formatting for legibility
	sub := bufferpool.Get()
	defer bufferpool.Put(sub)

	for ns, a := range attrs {
		sub.Reset()
		WriteTable(sub, a)

		if sub.Len() == 0 {
			continue
		}

		if buffer.Len() != 0 {
			buffer.WriteString(" | ")
		}

		io.WriteString(buffer, ns)
		io.WriteString(buffer, "::")
		sub.WriteTo(buffer)
	}
}
