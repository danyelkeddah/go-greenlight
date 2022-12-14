package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func (a *application) serve() error {
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", a.config.port),
		Handler:      a.routes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// receive errors returned by the graceful shutdown() function
	shutdownError := make(chan error)

	// start background routine
	go func() {
		// create a quite channel which caries os.Signal values
		quite := make(chan os.Signal, 1)

		// listen to SIGINT and SIGTERM signals and relay on them to quit channel, any other signals will not be caught and will retain their default behavior
		signal.Notify(quite, syscall.SIGINT, syscall.SIGTERM)
		// read the signal; block until we get signal
		s := <-quite
		// log message about the signal
		a.logger.PrintInfo("caught signal", map[string]string{
			"signal": s.String(),
		})

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		// will return nil if the graceful shutdown was successful
		// or an error (which may happen because of a problem closing the listeners, or because the shutdown didn't complete before the 20-second context deadline is hit
		err := srv.Shutdown(ctx)
		if err != nil {
			shutdownError <- err
		}
		a.logger.PrintInfo("completing background tasks", map[string]string{
			"addr": srv.Addr,
		})
		a.wg.Wait()
		shutdownError <- nil

	}()

	a.logger.PrintInfo("starting server", map[string]string{
		"addr": srv.Addr,
		"env":  a.config.env,
	})

	// Calling Shutdown() on our server will cause ListenAndServe() to immediately return a http.ErrServerClosed error,
	// So if we see this error, it is actually a good thing and an indication that the graceful shutdown has started.
	err := srv.ListenAndServe()

	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	// we wait to receive the return value from Shutdown() on the shutdown channel,
	// if return value is an error, we know that there was a problem with the graceful shutdown.
	err = <-shutdownError
	if err != nil {
		return err
	}

	a.logger.PrintInfo("stopped server", map[string]string{
		"addr": srv.Addr,
	})

	return nil
}
