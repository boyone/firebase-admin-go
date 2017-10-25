package db

import (
	"net/http"
	"reflect"
	"testing"
)

func TestGet(t *testing.T) {
	want := map[string]interface{}{"name": "Peter Parker", "age": float64(17)}
	mock := &mockServer{Resp: want}
	srv := mock.Start(client)
	defer srv.Close()

	var got map[string]interface{}
	if err := ref.Get(&got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("Get() = %v; want = %v", got, want)
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{Method: "GET", Path: "/peter.json"})
}

func TestGetWithStruct(t *testing.T) {
	want := person{Name: "Peter Parker", Age: 17}
	mock := &mockServer{Resp: want}
	srv := mock.Start(client)
	defer srv.Close()

	var got person
	if err := ref.Get(&got); err != nil {
		t.Fatal(err)
	}
	if want != got {
		t.Errorf("Get() = %v; want = %v", got, want)
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{Method: "GET", Path: "/peter.json"})
}

func TestGetWithETag(t *testing.T) {
	want := map[string]interface{}{"name": "Peter Parker", "age": float64(17)}
	mock := &mockServer{
		Resp:   want,
		Header: map[string]string{"ETag": "mock-etag"},
	}
	srv := mock.Start(client)
	defer srv.Close()

	var got map[string]interface{}
	etag, err := ref.GetWithETag(&got)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("Get() = %v; want = %v", got, want)
	}
	if etag != "mock-etag" {
		t.Errorf("ETag = %q; want = %q", etag, "mock-etag")
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{
		Method: "GET",
		Path:   "/peter.json",
		Header: http.Header{"X-Firebase-ETag": []string{"true"}},
	})
}

func TestGetIfChanged(t *testing.T) {
	want := map[string]interface{}{"name": "Peter Parker", "age": float64(17)}
	mock := &mockServer{
		Resp:   want,
		Header: map[string]string{"ETag": "new-etag"},
	}
	srv := mock.Start(client)
	defer srv.Close()

	var got map[string]interface{}
	ok, etag, err := ref.GetIfChanged("old-etag", &got)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Errorf("Get() = %v; want = %v", ok, true)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("Get() = %v; want = %v", got, want)
	}
	if etag != "new-etag" {
		t.Errorf("ETag = %q; want = %q", etag, "new-etag")
	}

	mock.Status = http.StatusNotModified
	mock.Resp = nil
	var got2 map[string]interface{}
	ok, etag, err = ref.GetIfChanged("new-etag", &got2)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Errorf("Get() = %v; want = %v", ok, false)
	}
	if got2 != nil {
		t.Errorf("Get() = %v; want nil", got2)
	}
	if etag != "new-etag" {
		t.Errorf("ETag = %q; want = %q", etag, "new-etag")
	}

	checkAllRequests(t, mock.Reqs, []*testReq{
		&testReq{
			Method: "GET",
			Path:   "/peter.json",
			Header: http.Header{"If-None-Match": []string{"old-etag"}},
		},
		&testReq{
			Method: "GET",
			Path:   "/peter.json",
			Header: http.Header{"If-None-Match": []string{"new-etag"}},
		},
	})
}

func TestWerlformedHttpError(t *testing.T) {
	mock := &mockServer{Resp: map[string]string{"error": "test error"}, Status: 500}
	srv := mock.Start(client)
	defer srv.Close()

	var got person
	err := ref.Get(&got)
	want := "http error status: 500; reason: test error"
	if err == nil || err.Error() != want {
		t.Errorf("Get() = %v; want = %v", err, want)
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{Method: "GET", Path: "/peter.json"})
}

func TestUnexpectedHttpError(t *testing.T) {
	mock := &mockServer{Resp: "unexpected error", Status: 500}
	srv := mock.Start(client)
	defer srv.Close()

	var got person
	err := ref.Get(&got)
	want := "http error status: 500; message: \"unexpected error\""
	if err == nil || err.Error() != want {
		t.Errorf("Get() = %v; want = %v", err, want)
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{Method: "GET", Path: "/peter.json"})
}

func TestSet(t *testing.T) {
	mock := &mockServer{}
	srv := mock.Start(client)
	defer srv.Close()

	want := map[string]interface{}{"name": "Peter Parker", "age": float64(17)}
	if err := ref.Set(want); err != nil {
		t.Fatal(err)
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{
		Method: "PUT",
		Path:   "/peter.json",
		Body:   serialize(want),
		Query:  map[string]string{"print": "silent"},
	})
}

func TestSetWithStruct(t *testing.T) {
	mock := &mockServer{}
	srv := mock.Start(client)
	defer srv.Close()

	want := &person{"Peter Parker", 17}
	if err := ref.Set(&want); err != nil {
		t.Fatal(err)
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{
		Method: "PUT",
		Path:   "/peter.json",
		Body:   serialize(want),
		Query:  map[string]string{"print": "silent"},
	})
}

func TestSetIfUnchanged(t *testing.T) {
	mock := &mockServer{}
	srv := mock.Start(client)
	defer srv.Close()

	want := &person{"Peter Parker", 17}
	ok, err := ref.SetIfUnchanged("mock-etag", &want)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Errorf("SetIfUnchanged() = %v; want = %v", ok, true)
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{
		Method: "PUT",
		Path:   "/peter.json",
		Body:   serialize(want),
		Header: http.Header{"If-Match": []string{"mock-etag"}},
	})
}

func TestSetIfUnchangedError(t *testing.T) {
	mock := &mockServer{
		Status: http.StatusPreconditionFailed,
		Resp:   &person{"Tony Stark", 39},
	}
	srv := mock.Start(client)
	defer srv.Close()

	want := &person{"Peter Parker", 17}
	ok, err := ref.SetIfUnchanged("mock-etag", &want)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Errorf("SetIfUnchanged() = %v; want = %v", ok, false)
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{
		Method: "PUT",
		Path:   "/peter.json",
		Body:   serialize(want),
		Header: http.Header{"If-Match": []string{"mock-etag"}},
	})
}

func TestPush(t *testing.T) {
	mock := &mockServer{Resp: map[string]string{"name": "new_key"}}
	srv := mock.Start(client)
	defer srv.Close()

	child, err := ref.Push(nil)
	if err != nil {
		t.Fatal(err)
	}

	if child.Key != "new_key" {
		t.Errorf("Push() = %q; want = %q", child.Key, "new_key")
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{
		Method: "POST",
		Path:   "/peter.json",
		Body:   serialize(""),
	})
}

func TestPushWithValue(t *testing.T) {
	mock := &mockServer{Resp: map[string]string{"name": "new_key"}}
	srv := mock.Start(client)
	defer srv.Close()

	want := map[string]interface{}{"name": "Peter Parker", "age": float64(17)}
	child, err := ref.Push(want)
	if err != nil {
		t.Fatal(err)
	}

	if child.Key != "new_key" {
		t.Errorf("Push() = %q; want = %q", child.Key, "new_key")
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{
		Method: "POST",
		Path:   "/peter.json",
		Body:   serialize(want),
	})
}

func TestUpdate(t *testing.T) {
	want := map[string]interface{}{"name": "Peter Parker", "age": float64(17)}
	mock := &mockServer{Resp: want}
	srv := mock.Start(client)
	defer srv.Close()

	if err := ref.Update(want); err != nil {
		t.Fatal(err)
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{
		Method: "PATCH",
		Path:   "/peter.json",
		Body:   serialize(want),
		Query:  map[string]string{"print": "silent"},
	})
}

func TestInvalidUpdate(t *testing.T) {
	if err := ref.Update(nil); err == nil {
		t.Errorf("Update(nil) = nil; want error")
	}

	m := make(map[string]interface{})
	if err := ref.Update(m); err == nil {
		t.Errorf("Update(map{}) = nil; want error")
	}
}

func TestTransaction(t *testing.T) {
	mock := &mockServer{
		Resp:   &person{"Peter Parker", 17},
		Header: map[string]string{"ETag": "mock-etag"},
	}
	srv := mock.Start(client)
	defer srv.Close()

	var fn UpdateFn = func(i interface{}) (interface{}, error) {
		p := i.(map[string]interface{})
		p["age"] = p["age"].(float64) + 1.0
		return p, nil
	}
	if err := ref.Transaction(fn); err != nil {
		t.Fatal(err)
	}
	checkAllRequests(t, mock.Reqs, []*testReq{
		&testReq{
			Method: "GET",
			Path:   "/peter.json",
			Header: http.Header{"X-Firebase-ETag": []string{"true"}},
		},
		&testReq{
			Method: "PUT",
			Path:   "/peter.json",
			Body: serialize(map[string]interface{}{
				"name": "Peter Parker",
				"age":  18,
			}),
			Header: http.Header{"If-Match": []string{"mock-etag"}},
		},
	})
}

func TestTransactionRetry(t *testing.T) {
	mock := &mockServer{
		Resp:   &person{"Peter Parker", 17},
		Header: map[string]string{"ETag": "mock-etag1"},
	}
	srv := mock.Start(client)
	defer srv.Close()

	cnt := 0
	var fn UpdateFn = func(i interface{}) (interface{}, error) {
		if cnt == 0 {
			mock.Status = http.StatusPreconditionFailed
			mock.Header = map[string]string{"ETag": "mock-etag2"}
			mock.Resp = &person{"Peter Parker", 19}
		} else if cnt == 1 {
			mock.Status = http.StatusOK
		}
		cnt++
		p := i.(map[string]interface{})
		p["age"] = p["age"].(float64) + 1.0
		return p, nil
	}
	if err := ref.Transaction(fn); err != nil {
		t.Fatal(err)
	}
	if cnt != 2 {
		t.Errorf("Retry Count = %d; want = %d", cnt, 2)
	}
	checkAllRequests(t, mock.Reqs, []*testReq{
		&testReq{
			Method: "GET",
			Path:   "/peter.json",
			Header: http.Header{"X-Firebase-ETag": []string{"true"}},
		},
		&testReq{
			Method: "PUT",
			Path:   "/peter.json",
			Body: serialize(map[string]interface{}{
				"name": "Peter Parker",
				"age":  18,
			}),
			Header: http.Header{"If-Match": []string{"mock-etag1"}},
		},
		&testReq{
			Method: "PUT",
			Path:   "/peter.json",
			Body: serialize(map[string]interface{}{
				"name": "Peter Parker",
				"age":  20,
			}),
			Header: http.Header{"If-Match": []string{"mock-etag2"}},
		},
	})
}

func TestTransactionAbort(t *testing.T) {
	mock := &mockServer{
		Resp:   &person{"Peter Parker", 17},
		Header: map[string]string{"ETag": "mock-etag1"},
	}
	srv := mock.Start(client)
	defer srv.Close()

	cnt := 0
	var fn UpdateFn = func(i interface{}) (interface{}, error) {
		if cnt == 0 {
			mock.Status = http.StatusPreconditionFailed
			mock.Header = map[string]string{"ETag": "mock-etag1"}
		}
		cnt++
		p := i.(map[string]interface{})
		p["age"] = p["age"].(float64) + 1.0
		return p, nil
	}
	err := ref.Transaction(fn)
	if err == nil {
		t.Errorf("Transaction() = nil; want error")
	}
	wanted := []*testReq{
		&testReq{
			Method: "GET",
			Path:   "/peter.json",
			Header: http.Header{"X-Firebase-ETag": []string{"true"}},
		},
	}
	for i := 0; i < 20; i++ {
		wanted = append(wanted, &testReq{
			Method: "PUT",
			Path:   "/peter.json",
			Body: serialize(map[string]interface{}{
				"name": "Peter Parker",
				"age":  18,
			}),
			Header: http.Header{"If-Match": []string{"mock-etag1"}},
		})
	}
	checkAllRequests(t, mock.Reqs, wanted)
}

func TestDelete(t *testing.T) {
	mock := &mockServer{Resp: "null"}
	srv := mock.Start(client)
	defer srv.Close()

	if err := ref.Delete(); err != nil {
		t.Fatal(err)
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{
		Method: "DELETE",
		Path:   "/peter.json",
	})
}