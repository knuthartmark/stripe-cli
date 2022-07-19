package cmd

import (
	"fmt"
	"net/http"

	"github.com/improbable-eng/grpc-web/go/grpcweb"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/stripe/stripe-cli/pkg/config"
	"github.com/stripe/stripe-cli/pkg/rpcservice"
	"github.com/stripe/stripe-cli/pkg/stripe"
	"github.com/stripe/stripe-cli/pkg/validators"
)

type daemonCmd struct {
	cmd  *cobra.Command
	port int
	http bool
	cfg  *config.Config
}

func newDaemonCmd(cfg *config.Config) *daemonCmd {
	dc := &daemonCmd{
		cfg: cfg,
	}

	dc.cmd = &cobra.Command{
		Use:   "daemon",
		Args:  validators.NoArgs,
		Short: "Run as a daemon on your localhost",
		Long: `Start a local gRPC server, enabling you to invoke Stripe CLI commands programmatically from a gRPC
client.

Currently, stripe daemon only supports a subset of CLI commands. Documentation is not yet available.`,
		Run:    dc.runDaemonCmd,
		Hidden: true,
	}
	dc.cmd.Flags().IntVar(&dc.port, "port", 0, "The TCP port the daemon will listen to (default: an available port)")
	dc.cmd.Flags().BoolVar(&dc.http, "http", false, "Spin up an http-compatible gRPC service (default: false)")

	return dc
}

func (dc *daemonCmd) runDaemonCmd(cmd *cobra.Command, args []string) {
	telemetryClient := stripe.GetTelemetryClient(cmd.Context())
	srv := rpcservice.New(&rpcservice.Config{
		Port:    dc.port,
		Log:     log.StandardLogger(),
		UserCfg: dc.cfg,
	}, telemetryClient)

	ctx := withSIGTERMCancel(cmd.Context(), func() {
		log.WithFields(log.Fields{
			"prefix": "cmd.daemonCmd.runDaemonCmd",
		}).Debug("Ctrl+C received, cleaning up...")
	})

	go srv.Run(ctx)

	if dc.http {
		grpcWeb := grpcweb.WrapServer(
			srv.Server(),
			grpcweb.WithOriginFunc(func(origin string) bool { return true }),
		)

		webSrv := &http.Server{
			Handler: grpcWeb,
			Addr:    fmt.Sprintf("localhost:%d", dc.port+1),
		}

		go func() {
			if err := webSrv.ListenAndServe(); err != nil {
				log.Fatalf("failed to serve: %v", err)
			}
		}()
	}

	<-ctx.Done()
}
