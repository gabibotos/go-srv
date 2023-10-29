package log

type Logger interface {
	Printf(string, ...interface{})
	Fatalf(string, ...interface{})
}
