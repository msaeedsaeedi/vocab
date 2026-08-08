// Package instance prevents multiple desktop daemon instances from running.
package instance

// Acquire obtains the application singleton. The returned release function
// must be called by the owner before it exits. alreadyRunning is true when a
// different process already owns the singleton.
func Acquire() (release func(), alreadyRunning bool, err error) {
	return acquire()
}
