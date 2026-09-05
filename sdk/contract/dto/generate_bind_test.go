package dto

import (
	"bytes"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
)

func deleteCtx(t *testing.T, uriID, body string) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("DELETE", "/x/"+uriID, bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: uriID}}
	return c
}

// Binding through &s handed gin a **ObjectById. A body of `null` then set
// that inner pointer to nil, and gin's validator panicked reaching through
// it - a request body a client fully controls taking the handler down.
func TestObjectByIdBindSurvivesANullBody(t *testing.T) {
	s := &ObjectById{}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("a null body panicked: %v", r)
		}
	}()
	_ = s.Bind(deleteCtx(t, "1", "null"))
}

func TestObjectDeleteReqBindSurvivesANullBody(t *testing.T) {
	s := &ObjectDeleteReq{}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("a null body panicked: %v", r)
		}
	}()
	_ = s.Bind(deleteCtx(t, "1", "null"))
}

func TestObjectByIdBindKeepsBothTheUriIdAndTheBodyIds(t *testing.T) {
	s := &ObjectById{}
	if err := s.Bind(deleteCtx(t, "7", `{"ids":[2,3]}`)); err != nil {
		t.Fatalf("bind: %v", err)
	}
	if s.Id != 7 {
		t.Fatalf("Id = %d, want the id bound from the URI (7)", s.Id)
	}
	if !reflect.DeepEqual(s.Ids, []int{2, 3}) {
		t.Fatalf("Ids = %v, want [2 3]", s.Ids)
	}
}

func TestObjectByIdBindFoldsTheUriIdIntoIdsWhenTheBodyCarriesNone(t *testing.T) {
	s := &ObjectById{}
	if err := s.Bind(deleteCtx(t, "7", `{}`)); err != nil {
		t.Fatalf("bind: %v", err)
	}
	if !reflect.DeepEqual(s.Ids, []int{7}) {
		t.Fatalf("Ids = %v, want [7]", s.Ids)
	}
}

// GetId used to append to s.Ids in place, so calling it twice returned the
// URI id twice - and every caller that reads a request twice, or retries,
// widened the delete each time.
func TestGetIdIsIdempotent(t *testing.T) {
	s := &ObjectById{Id: 7, Ids: []int{2, 3}}
	first := s.GetId()
	second := s.GetId()
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("GetId returned %v then %v", first, second)
	}
	if !reflect.DeepEqual(s.Ids, []int{2, 3}) {
		t.Fatalf("GetId wrote back to the receiver: Ids = %v, want [2 3]", s.Ids)
	}
}

// A route with no :id leaves Id at 0. Appending it put a 0 in the slice,
// which a caller treating 0 as "unset" can read as something else entirely.
func TestGetIdOmitsAZeroUriId(t *testing.T) {
	s := &ObjectById{Id: 0, Ids: []int{2, 3}}
	got := s.GetId()
	if !reflect.DeepEqual(got, []int{2, 3}) {
		t.Fatalf("GetId = %v, want [2 3]", got)
	}
}

func TestGetIdReturnsTheSingleIdWhenTheBodyCarriesNone(t *testing.T) {
	s := &ObjectById{Id: 7}
	if got := s.GetId(); got != 7 {
		t.Fatalf("GetId = %v, want 7", got)
	}
}
