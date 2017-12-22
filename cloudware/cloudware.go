package cloudware

type (
	CLIFlags struct {
		Addr *string
	}

	CLIService interface {
		ParseFlags(version string) (*CLIFlags, error)
		ValidateFlags(flags *CLIFlags) error
	}
)
