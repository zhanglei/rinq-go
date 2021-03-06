package options_test

import (
	"os"
	"time"

	"github.com/jmalloc/twelf/src/twelf"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rinq/rinq-go/src/rinq/options"
)

var _ = Describe("FromEnv", func() {
	AfterEach(func() {
		os.Setenv("RINQ_DEFAULT_TIMEOUT", "")
		os.Setenv("RINQ_LOG_DEBUG", "")
		os.Setenv("RINQ_COMMAND_WORKERS", "")
		os.Setenv("RINQ_SESSION_WORKERS", "")
		os.Setenv("RINQ_PRUNE_INTERVAL", "")
		os.Setenv("RINQ_PRODUCT", "")
	})

	It("returns an empty slice when no environment variables are set", func() {
		o, err := options.FromEnv()

		Expect(err).NotTo(HaveOccurred())
		Expect(o).To(HaveLen(0))
	})

	Context("RINQ_DEFAULT_TIMEOUT", func() {
		It("returns a DefaultTimeout option", func() {
			os.Setenv("RINQ_DEFAULT_TIMEOUT", "500")
			o, err := options.FromEnv()

			Expect(err).NotTo(HaveOccurred())

			opts, err := options.NewOptions(o...)

			Expect(err).NotTo(HaveOccurred())
			Expect(opts.DefaultTimeout).To(Equal(500 * time.Millisecond))
		})

		It("returns an error if the value is not a positive integer", func() {
			os.Setenv("RINQ_DEFAULT_TIMEOUT", "-500")
			_, err := options.FromEnv()

			Expect(err).To(HaveOccurred())
		})
	})

	Context("RINQ_LOG_DEBUG", func() {
		It("returns a Logger option when set to true", func() {
			os.Setenv("RINQ_LOG_DEBUG", "true")
			o, err := options.FromEnv()

			Expect(err).NotTo(HaveOccurred())

			opts, err := options.NewOptions(o...)

			Expect(err).NotTo(HaveOccurred())
			Expect(opts.Logger).To(Equal(
				&twelf.StandardLogger{CaptureDebug: true},
			))
		})

		It("returns a Logger option when set to false", func() {
			os.Setenv("RINQ_LOG_DEBUG", "false")
			o, err := options.FromEnv()

			Expect(err).NotTo(HaveOccurred())

			opts, err := options.NewOptions(o...)

			Expect(err).NotTo(HaveOccurred())
			Expect(opts.Logger).To(Equal(
				&twelf.StandardLogger{CaptureDebug: false},
			))
		})

		It("returns an error if the value is not a boolean", func() {
			os.Setenv("RINQ_LOG_DEBUG", "invalid")
			_, err := options.FromEnv()

			Expect(err).To(HaveOccurred())
		})
	})

	Context("RINQ_COMMAND_WORKERS", func() {
		It("returns a CommandWorkers option", func() {
			os.Setenv("RINQ_COMMAND_WORKERS", "15")
			o, err := options.FromEnv()

			Expect(err).NotTo(HaveOccurred())

			opts, err := options.NewOptions(o...)

			Expect(err).NotTo(HaveOccurred())
			Expect(opts.CommandWorkers).To(Equal(uint(15)))
		})

		It("returns an error if the value is not a positive integer", func() {
			os.Setenv("RINQ_COMMAND_WORKERS", "-500")
			_, err := options.FromEnv()

			Expect(err).To(HaveOccurred())
		})
	})

	Context("RINQ_SESSION_WORKERS", func() {
		It("returns a SessionWorkers option", func() {
			os.Setenv("RINQ_SESSION_WORKERS", "25")
			o, err := options.FromEnv()

			Expect(err).NotTo(HaveOccurred())

			opts, err := options.NewOptions(o...)

			Expect(err).NotTo(HaveOccurred())
			Expect(opts.SessionWorkers).To(Equal(uint(25)))
		})

		It("returns an error if the value is not a positive integer", func() {
			os.Setenv("RINQ_SESSION_WORKERS", "-500")
			_, err := options.FromEnv()

			Expect(err).To(HaveOccurred())
		})
	})

	Context("RINQ_PRUNE_INTERVAL", func() {
		It("returns a PruneInterval option", func() {
			os.Setenv("RINQ_PRUNE_INTERVAL", "1500")
			o, err := options.FromEnv()

			Expect(err).NotTo(HaveOccurred())

			opts, err := options.NewOptions(o...)

			Expect(err).NotTo(HaveOccurred())
			Expect(opts.PruneInterval).To(Equal(1500 * time.Millisecond))
		})

		It("returns an error if the value is not a positive integer", func() {
			os.Setenv("RINQ_PRUNE_INTERVAL", "-500")
			_, err := options.FromEnv()

			Expect(err).To(HaveOccurred())
		})
	})

	Context("RINQ_PRODUCT", func() {
		It("returns a Product option", func() {
			os.Setenv("RINQ_PRODUCT", "my-app")
			o, err := options.FromEnv()

			Expect(err).NotTo(HaveOccurred())

			opts, err := options.NewOptions(o...)

			Expect(err).NotTo(HaveOccurred())
			Expect(opts.Product).To(Equal("my-app"))
		})
	})
})
