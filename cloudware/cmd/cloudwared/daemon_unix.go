// +build !windows

package main

// notifyShutdown is called after the daemon shuts down but before the process exits.
func notifyShutdown(err error) {
}
