package zap

import (
	"fmt"
	"testing"

	"github.com/go-admin-team/go-admin-core/debug/writer"
	"github.com/go-admin-team/go-admin-core/logger"
)

func TestName(t *testing.T) {
	l, err := NewLogger()
	if err != nil {
		t.Fatal(err)
	}

	if l.String() != "zap" {
		t.Errorf("name is error %s", l.String())
	}

	t.Logf("test logger name: %s", l.String())
}

func TestLogf(t *testing.T) {
	l, err := NewLogger()
	if err != nil {
		t.Fatal(err)
	}

	logger.DefaultLogger = l
	logger.Logf(logger.InfoLevel, "test logf: %s", "name")
}

func TestSetLevel(t *testing.T) {
	l, err := NewLogger()
	if err != nil {
		t.Fatal(err)
	}
	logger.DefaultLogger = l

	logger.Init(logger.WithLevel(logger.DebugLevel))
	l.Logf(logger.DebugLevel, "test show debug: %s", "debug msg")

	logger.Init(logger.WithLevel(logger.InfoLevel))
	l.Logf(logger.DebugLevel, "test non-show debug: %s", "debug msg")
}

func TestWithReportCaller(t *testing.T) {
	var err error
	logger.DefaultLogger, err = NewLogger(WithCallerSkip(0))
	if err != nil {
		t.Fatal(err)
	}

	logger.Logf(logger.InfoLevel, "testing: %s", "WithReportCaller")
}

func TestFields(t *testing.T) {
	l, err := NewLogger()
	if err != nil {
		t.Fatal(err)
	}
	logger.DefaultLogger = l.Fields(map[string]interface{}{
		"x-request-id": "123456abc",
	})
	logger.DefaultLogger.Log(logger.InfoLevel, "hello")
}

func TestFile(t *testing.T) {
	output, err := writer.NewFileWriter(writer.WithPath("testdata"), writer.WithSuffix("log"))
	if err != nil {
		t.Errorf("logger setup error: %s", err.Error())
	}
	//var err error
	logger.DefaultLogger, err = NewLogger(logger.WithLevel(logger.TraceLevel), WithOutput(output))
	if err != nil {
		t.Errorf("logger setup error: %s", err.Error())
	}
	logger.DefaultLogger = logger.DefaultLogger.Fields(map[string]interface{}{
		"x-request-id": "123456abc",
	})
	fmt.Println(logger.DefaultLogger)
	logger.DefaultLogger.Log(logger.InfoLevel, "hello")
}

//func TestFileKeep(t *testing.T) {
//	output, err := writer.NewFileWriter(writer.WithPath("testdata"), writer.WithSuffix("log"))
//	if err != nil {
//		t.Errorf("logger setup error: %s", err.Error())
//	}
//	//var err error
//	logger.DefaultLogger, err = NewLogger(logger.WithLevel(logger.TraceLevel), WithOutput(output))
//	if err != nil {
//		t.Errorf("logger setup error: %s", err.Error())
//	}
//
//	fmt.Println(logger.DefaultLogger)
//	logger.
//}
