package bootstrap

import (
	"os"
	"xconfig"
	"xlog"

	"github.com/go-kit/kit/log"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.uber.org/fx"
)

// Environment supplies the externally created components and configurable options for bootstrap an fx application
// using spf13's flagset and viper libraries together with go-kit logging.
type Environment struct {
	// Name is the application name, typically the executable name.  If unset, os.Args[0] is used.
	//
	// The name is what is passed to NewFlagSet and is the same value passed to Initialize.
	Name string

	// Arguments are the command-line arguments to be parsed via the spf13/pflag package.  If unset, os.Args[1:] is used.
	Arguments []string

	// ErrorHandling is the pflag error handling strategy.  By default, this is ContinueOnError.
	ErrorHandling pflag.ErrorHandling

	// LogKey is the viper configuration key where logging configuration is supplied.
	// There is no default.  If unset, xlog.Default() is used as the logger.
	LogKey string

	// DecodeOptions are the optional Viper options for unmarshalling.  These are used when emitting
	// the various Viper-related components and when unmarshalling the logger.
	DecodeOptions []viper.DecoderConfigOption

	// FlagSetBuilder is an optional closure that builds command line options.  This closure can return
	// an arbitrary type, such as a pointer to struct, that will contain the results of parsing the command line.
	// If not supplied, no setup is performed.
	//
	// This closure should not parse the command-line itself.  That is done by the Bootstrap method.
	FlagSetBuilder func(*pflag.FlagSet) (interface{}, error)

	// Initialize is the optionsl closure used to initialize the environment.  This function should configure
	// the flagset and viper, parse the command line, and read in configuration as appropriate.  It represents
	// the application layer's specific code to bootstrap the environment.  The interface{} parameter is the
	// value returned by FlagSetBuilder, or nil if that closure was not specified.
	//
	// If not supplied, no viper setup is performed.
	Initialize func(string, interface{}, *pflag.FlagSet, *viper.Viper) error
}

// newErrorOption produces an uber/fx Option which discards container printing and emits
// the given error from an Invoke function.  Handy when some fatal error has occurred during
// bootstrapping and that error should be available via fx.App.Err().
func newErrorOption(err error) fx.Option {
	return fx.Options(
		fx.Logger(xlog.Printer{Logger: xlog.Discard()}),
		fx.Invoke(func() error { return err }),
	)
}

// Bootstrap creates the infrastructure that needs to exist before the container is created, e.g. with fx.New.
//
// This function does the following:
//   - Creates pflag.FlagSet and viper.Viper instances
//   - Invokes the Initialize closure to allow application code to setup the environment
//   - Unmarshals logging configuration from the viper instance and creates a go-kit logger
//   - Sets the logger as the uber/fx printer
//   - Provides the flagset, viper, and logger instances as application components
//
// Any errors that occur during bootstrapping are emitted as fx.Invoke functions and will be available via App.Err().
// For example, if the help options was requested on the command line, pflag.ErrHelp will be returned to the application.
func (e Environment) Bootstrap() fx.Option {
	name := e.Name
	if len(name) == 0 {
		name = os.Args[0]
	}

	arguments := e.Arguments
	if arguments == nil {
		arguments = os.Args[1:]
	}

	var (
		flagSet     = pflag.NewFlagSet(name, e.ErrorHandling)
		viper       = viper.New()
		commandLine interface{}
	)

	if e.FlagSetBuilder != nil {
		var err error
		if commandLine, err = e.FlagSetBuilder(flagSet); err != nil {
			return newErrorOption(err)
		}
	}

	if err := flagSet.Parse(arguments); err != nil {
		return newErrorOption(err)
	}

	if e.Initialize != nil {
		if err := e.Initialize(name, commandLine, flagSet, viper); err != nil {
			return newErrorOption(err)
		}
	}

	logger := xlog.Default()
	if len(e.LogKey) > 0 {
		var err error
		logger, err = xlog.Unmarshal(e.LogKey, xconfig.ViperUnmarshaller{Viper: viper, Options: e.DecodeOptions})
		if err != nil {
			return newErrorOption(err)
		}
	}

	return fx.Options(
		fx.Logger(xlog.Printer{Logger: logger}),
		fx.Provide(
			func() log.Logger { return logger },
			func() *pflag.FlagSet { return flagSet },
			xconfig.ProvideViper(viper, e.DecodeOptions...),
		),
	)
}
