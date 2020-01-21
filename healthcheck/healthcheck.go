package healthcheck

import (
	"context"
	"errors"
	"runtime"
	"time"
)

const language = "go"

// Checker represents the interface all checker functions abide to
type Checker func(context.Context) (*Check, error)

// Check represents the details of a single checked dependency
type Check struct {
	Name        string     `json:"name"`
	Status      string     `json:"status"`
	StatusCode  int        `json:"status_code,omitempty"`
	Message     string     `json:"message"`
	LastChecked *time.Time `json:"last_checked"`
	LastSuccess *time.Time `json:"last_success"`
	LastFailure *time.Time `json:"last_failure"`
}

// HealthCheck represents the structure of the current health of a service/app
type HealthCheck struct {
	Status                   string        `json:"status"`
	Version                  VersionInfo   `json:"version"`
	Uptime                   time.Duration `json:"uptime"`
	StartTime                time.Time     `json:"start_time"`
	Checks                   []Check       `json:"checks"`
	Started                  bool          `json:"-"`
	Interval                 time.Duration `json:"-"`
	Clients                  []*Client     `json:"-"`
	CriticalErrorTimeout     time.Duration `json:"-"`
	TimeOfFirstCriticalError time.Time     `json:"-"`
	Tickers                  []*ticker     `json:"-"`
}

// VersionInfo represents the version information of service/app
type VersionInfo struct {
	BuildTime       time.Time `json:"build_time"`
	GitCommit       string    `json:"git_commit"`
	Language        string    `json:"language"`
	LanguageVersion string    `json:"language_version"`
	Version         string    `json:"version"`
}

// Create returns a new instantiated HealthCheck object. Caller to provide:
// version information of the app,
// criticalTimeout for how long to wait until an unhealthy dependent propagates its state to make this app unhealthy
// interval in which to check health of dependencies
// checkers of type Checker which are functions run to check the health of the app and/or its dependencies
func Create(version VersionInfo, criticalTimeout, interval time.Duration, checkers ...*Checker) HealthCheck {

	var clients []*Client

	hc := HealthCheck{
		Started:              false,
		Clients:              clients,
		Version:              version,
		CriticalErrorTimeout: criticalTimeout,
		Interval:             interval,
	}

	for _, checker := range checkers {
		hc.AddCheck(checker)
	}

	return hc
}

// CreateVersionInfo returns a health check version info object. Caller to provide:
// buildTime for when the app was built
// gitCommit the SHA-1 commit hash of the built app
// version the semantic version of the built app
func CreateVersionInfo(buildTime time.Time, gitCommit, version string) VersionInfo {
	return VersionInfo{
		BuildTime:       buildTime,
		GitCommit:       gitCommit,
		Language:        language,
		LanguageVersion: runtime.Version(),
		Version:         version,
	}
}

// AddCheck adds a provided checker to the healthcheck
func (hc *HealthCheck) AddCheck(checker *Checker) (err error) {
	if hc.Started {
		err := errors.New("unable to add new client, health check has already started")
		return err
	}

	client, err := newClient(checker)
	if err != nil {
		return err
	}

	hc.Clients = append(hc.Clients, client)

	return nil
}

// newTickers returns an array of tickers based on the number of clients in the clients parameter.
// Each client is executed at the given interval also passed into the function
func newTickers(interval time.Duration, clients []*Client) []*ticker {
	var tickers []*ticker
	for _, client := range clients {
		tickers = append(tickers, createTicker(interval, client))
	}
	return tickers
}

// Start begins each ticker, this is used to run the health checks on dependent apps
// takes argument context and should utilise contextWithCancel
func (hc *HealthCheck) Start(ctx context.Context) {
	hc.Started = true
	hc.Tickers = newTickers(hc.Interval, hc.Clients)
	hc.StartTime = time.Now().UTC()
	for _, ticker := range hc.Tickers {
		ticker.start(ctx)
	}
}

// Stop will cancel all tickers and thus stop all health checks
func (hc *HealthCheck) Stop() {
	for _, ticker := range hc.Tickers {
		ticker.stop()
	}
}
