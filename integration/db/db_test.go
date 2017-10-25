package db

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"testing"

	"firebase.google.com/go"

	"io/ioutil"

	"encoding/json"

	"reflect"

	"bytes"

	"firebase.google.com/go/db"
	"firebase.google.com/go/integration/internal"
)

var client *db.Client
var aoClient *db.Client
var guestClient *db.Client

var ref *db.Ref
var users *db.Ref
var dinos *db.Ref

var testData map[string]interface{}
var parsedTestData map[string]Dinosaur

const permDenied = "http error status: 401; reason: Permission denied"

func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		log.Println("skipping database integration tests in short mode.")
		os.Exit(0)
	}

	pid, err := internal.ProjectID()
	if err != nil {
		log.Fatalln(err)
	}

	client, err = initClient(pid)
	if err != nil {
		log.Fatalln(err)
	}

	aoClient, err = initOverrideClient(pid)
	if err != nil {
		log.Fatalln(err)
	}

	initRefs()
	initRules()
	initData()

	os.Exit(m.Run())
}

func initClient(pid string) (*db.Client, error) {
	ctx := context.Background()
	app, err := internal.NewTestApp(ctx, &firebase.Config{
		DatabaseURL: fmt.Sprintf("https://%s.firebaseio.com", pid),
	})
	if err != nil {
		return nil, err
	}

	return app.Database(ctx)
}

func initOverrideClient(pid string) (*db.Client, error) {
	ctx := context.Background()
	app, err := internal.NewTestApp(ctx, &firebase.Config{
		DatabaseURL:   fmt.Sprintf("https://%s.firebaseio.com", pid),
		AuthOverrides: map[string]interface{}{"uid": "user1"},
	})
	if err != nil {
		return nil, err
	}

	return app.Database(ctx)
}

func initRefs() {
	var err error
	ref, err = client.NewRef("_adminsdk/go/dinodb")
	if err != nil {
		log.Fatalln(err)
	}

	dinos, err = ref.Child("dinosaurs")
	if err != nil {
		log.Fatalln(err)
	}

	users, err = ref.Parent().Child("users")
	if err != nil {
		log.Fatalln(err)
	}
}

func initRules() {
	b, err := ioutil.ReadFile(internal.Resource("dinosaurs_index.json"))
	if err != nil {
		log.Fatalln(err)
	}

	pid, err := internal.ProjectID()
	if err != nil {
		log.Fatalln(err)
	}

	url := fmt.Sprintf("https://%s.firebaseio.com/.settings/rules.json", pid)
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(b))
	if err != nil {
		log.Fatalln(err)
	}
	req.Header.Set("Content-Type", "application/json")

	hc, err := internal.NewHTTPClient(context.Background())
	if err != nil {
		log.Fatalln(err)
	}
	resp, err := hc.Do(req)
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()

	b, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	} else if resp.StatusCode != http.StatusOK {
		log.Fatalln("failed to update rules: %q", string(b))
	}
}

func initData() {
	b, err := ioutil.ReadFile(internal.Resource("dinosaurs.json"))
	if err != nil {
		log.Fatalln(err)
	}
	if err = json.Unmarshal(b, &testData); err != nil {
		log.Fatalln(err)
	}

	b, err = json.Marshal(testData["dinosaurs"])
	if err != nil {
		log.Fatalln(err)
	}
	if err = json.Unmarshal(b, &parsedTestData); err != nil {
		log.Fatalln(err)
	}

	if err = ref.Set(testData); err != nil {
		log.Fatalln(err)
	}
}

func TestRef(t *testing.T) {
	if ref.Key != "dinodb" {
		t.Errorf("Key = %q; want = %q", ref.Key, "dinodb")
	}
	if ref.Path != "/_adminsdk/go/dinodb" {
		t.Errorf("Path = %q; want = %q", ref.Path, "/_adminsdk/go/dinodb")
	}
}

func TestChild(t *testing.T) {
	c, err := ref.Child("dinosaurs")
	if err != nil {
		t.Fatal(err)
	}
	if c.Key != "dinosaurs" {
		t.Errorf("Key = %q; want = %q", c.Key, "dinosaurs")
	}
	if c.Path != "/_adminsdk/go/dinodb/dinosaurs" {
		t.Errorf("Path = %q; want = %q", c.Path, "/_adminsdk/go/dinodb/dinosaurs")
	}
}

func TestParent(t *testing.T) {
	p := ref.Parent()
	if p.Key != "go" {
		t.Errorf("Key = %q; want = %q", p.Key, "go")
	}
	if p.Path != "/_adminsdk/go" {
		t.Errorf("Path = %q; want = %q", p.Path, "/_adminsdk/go")
	}
}

func TestGet(t *testing.T) {
	var m map[string]interface{}
	if err := ref.Get(&m); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(testData, m) {
		t.Errorf("Get() = %v; want = %v", m, testData)
	}
}

func TestGetWithETag(t *testing.T) {
	var m map[string]interface{}
	etag, err := ref.GetWithETag(&m)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(testData, m) {
		t.Errorf("GetWithETag() = %v; want = %v", m, testData)
	}
	if etag == "" {
		t.Errorf("GetWithETag() = \"\"; want non-empty")
	}
}

func TestGetIfChanged(t *testing.T) {
	var m map[string]interface{}
	ok, etag, err := ref.GetIfChanged("wrong-etag", &m)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || etag == "" {
		t.Errorf("GetIfChanged() = (%v, %q); want = (%v, %q)", ok, etag, true, "non-empty")
	}
	if !reflect.DeepEqual(testData, m) {
		t.Errorf("GetWithETag() = %v; want = %v", m, testData)
	}

	var m2 map[string]interface{}
	ok, etag2, err := ref.GetIfChanged(etag, &m2)
	if err != nil {
		t.Fatal(err)
	}
	if ok || etag != etag2 {
		t.Errorf("GetIfChanged() = (%v, %q); want = (%v, %q)", ok, etag2, false, etag)
	}
	if len(m2) != 0 {
		t.Errorf("GetWithETag() = %v; want empty", m)
	}
}

func TestGetChildValue(t *testing.T) {
	c, err := ref.Child("dinosaurs")
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]interface{}
	if err := c.Get(&m); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(testData["dinosaurs"], m) {
		t.Errorf("Get() = %v; want = %v", m, testData["dinosaurs"])
	}
}

func TestGetGrandChildValue(t *testing.T) {
	c, err := ref.Child("dinosaurs/lambeosaurus")
	if err != nil {
		t.Fatal(err)
	}

	var got Dinosaur
	if err := c.Get(&got); err != nil {
		t.Fatal(err)
	}
	want := parsedTestData["lambeosaurus"]
	if !reflect.DeepEqual(want, got) {
		t.Errorf("Get() = %v; want = %v", got, want)
	}
}

func TestGetNonExistingChild(t *testing.T) {
	c, err := ref.Child("non_existing")
	if err != nil {
		t.Fatal(err)
	}

	var i interface{}
	if err := c.Get(&i); err != nil {
		t.Fatal(err)
	}
	if i != nil {
		t.Errorf("Get() = %v; want nil", i)
	}
}

func TestPush(t *testing.T) {
	u, err := users.Push(nil)
	if err != nil {
		t.Fatal(err)
	}
	if u.Path != "/_adminsdk/go/users/"+u.Key {
		t.Errorf("Push() = %q; want = %q", u.Path, "/_adminsdk/go/users/"+u.Key)
	}

	var i interface{}
	if err := u.Get(&i); err != nil {
		t.Fatal(err)
	}
	if i != "" {
		t.Errorf("Get() = %v; want empty string", i)
	}
}

func TestPushWithValue(t *testing.T) {
	want := User{"Luis Alvarez", 1911}
	u, err := users.Push(&want)
	if err != nil {
		t.Fatal(err)
	}
	if u.Path != "/_adminsdk/go/users/"+u.Key {
		t.Errorf("Push() = %q; want = %q", u.Path, "/_adminsdk/go/users/"+u.Key)
	}

	var got User
	if err := u.Get(&got); err != nil {
		t.Fatal(err)
	}
	if want != got {
		t.Errorf("Get() = %v; want = %v", got, want)
	}
}

func TestSetPrimitiveValue(t *testing.T) {
	u, err := users.Push(nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := u.Set("value"); err != nil {
		t.Fatal(err)
	}
	var got string
	if err := u.Get(&got); err != nil {
		t.Fatal(err)
	}
	if got != "value" {
		t.Errorf("Get() = %q; want = %q", got, "value")
	}
}

func TestSetComplexValue(t *testing.T) {
	u, err := users.Push(nil)
	if err != nil {
		t.Fatal(err)
	}

	want := User{"Mary Anning", 1799}
	if err := u.Set(&want); err != nil {
		t.Fatal(err)
	}
	var got User
	if err := u.Get(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("Get() = %v; want = %v", got, want)
	}
}

func TestUpdateChildren(t *testing.T) {
	u, err := users.Push(nil)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]interface{}{
		"name":  "Robert Bakker",
		"since": float64(1945),
	}
	if err := u.Update(want); err != nil {
		t.Fatal(err)
	}
	var got map[string]interface{}
	if err := u.Get(&got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("Get() = %v; want = %v", got, want)
	}
}

func TestUpdateChildrenWithExistingValue(t *testing.T) {
	u, err := users.Push(map[string]interface{}{
		"name":  "Edwin Colbert",
		"since": float64(1900),
	})
	if err != nil {
		t.Fatal(err)
	}

	update := map[string]interface{}{"since": float64(1905)}
	if err := u.Update(update); err != nil {
		t.Fatal(err)
	}
	var got map[string]interface{}
	if err := u.Get(&got); err != nil {
		t.Fatal(err)
	}
	want := map[string]interface{}{
		"name":  "Edwin Colbert",
		"since": float64(1905),
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("Get() = %v; want = %v", got, want)
	}
}

func TestUpdateNestedChildren(t *testing.T) {
	edward, err := users.Push(map[string]interface{}{"name": "Edward Cope", "since": float64(1800)})
	if err != nil {
		t.Fatal(err)
	}
	jack, err := users.Push(map[string]interface{}{"name": "Jack Horner", "since": float64(1940)})
	if err != nil {
		t.Fatal(err)
	}
	delta := map[string]interface{}{
		fmt.Sprintf("%s/since", edward.Key): 1840,
		fmt.Sprintf("%s/since", jack.Key):   1946,
	}
	if err := users.Update(delta); err != nil {
		t.Fatal(err)
	}
	var got map[string]interface{}
	if err := edward.Get(&got); err != nil {
		t.Fatal(err)
	}
	want := map[string]interface{}{"name": "Edward Cope", "since": float64(1840)}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("Get() = %v; want = %v", got, want)
	}

	if err := jack.Get(&got); err != nil {
		t.Fatal(err)
	}
	want = map[string]interface{}{"name": "Jack Horner", "since": float64(1946)}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("Get() = %v; want = %v", got, want)
	}
}

func TestSetIfChanged(t *testing.T) {
	edward, err := users.Push(&User{"Edward Cope", 1800})
	if err != nil {
		t.Fatal(err)
	}

	update := User{"Jack Horner", 1940}
	ok, err := edward.SetIfUnchanged("invalid-etag", &update)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Errorf("SetIfUnchanged() = %v; want = %v", ok, false)
	}

	var u User
	etag, err := edward.GetWithETag(&u)
	if err != nil {
		t.Fatal(err)
	}
	ok, err = edward.SetIfUnchanged(etag, &update)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Errorf("SetIfUnchanged() = %v; want = %v", ok, true)
	}

	if err := edward.Get(&u); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(update, u) {
		t.Errorf("Get() = %v; want = %v", u, update)
	}
}

func TestTransaction(t *testing.T) {
	u, err := users.Push(&User{Name: "Richard"})
	if err != nil {
		t.Fatal(err)
	}
	fn := func(curr interface{}) (interface{}, error) {
		snap := curr.(map[string]interface{})
		snap["name"] = "Richard Owen"
		snap["since"] = 1804
		return snap, nil
	}
	if err := u.Transaction(fn); err != nil {
		t.Fatal(err)
	}
	var got User
	if err := u.Get(&got); err != nil {
		t.Fatal(err)
	}
	want := User{"Richard Owen", 1804}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("Get() = %v; want = %v", got, want)
	}
}

func TestTransactionScalar(t *testing.T) {
	cnt, err := users.Child("count")
	if err != nil {
		t.Fatal(err)
	}
	if err := cnt.Set(42); err != nil {
		t.Fatal(err)
	}
	fn := func(curr interface{}) (interface{}, error) {
		snap := curr.(float64)
		return snap + 1, nil
	}
	if err := cnt.Transaction(fn); err != nil {
		t.Fatal(err)
	}
	var got float64
	if err := cnt.Get(&got); err != nil {
		t.Fatal(err)
	}
	if got != 43.0 {
		t.Errorf("Get() = %v; want = %v", got, 43.0)
	}
}

func TestDelete(t *testing.T) {
	u, err := users.Push("foo")
	if err != nil {
		t.Fatal(err)
	}
	var got string
	if err := u.Get(&got); err != nil {
		t.Fatal(err)
	}
	if got != "foo" {
		t.Errorf("Get() = %q; want = %q", got, "foo")
	}
	if err := u.Delete(); err != nil {
		t.Fatal(err)
	}

	var got2 string
	if err := u.Get(&got2); err != nil {
		t.Fatal(err)
	}
	if got2 != "" {
		t.Errorf("Get() = %q; want = %q", got2, "")
	}
}

func TestNoAccess(t *testing.T) {
	r, err := aoClient.NewRef(protectedRef(t, "_adminsdk/go/admin"))
	if err != nil {
		t.Fatal(err)
	}
	var got string
	if err := r.Get(&got); err == nil || got != "" {
		t.Errorf("Get() = (%q, %v); want = (empty, error)", got, err)
	} else if err.Error() != permDenied {
		t.Errorf("Error = %q; want = %q", err.Error(), permDenied)
	}
	if err := r.Set("update"); err == nil {
		t.Errorf("Set() = nil; want = error")
	} else if err.Error() != permDenied {
		t.Errorf("Error = %q; want = %q", err.Error(), permDenied)
	}
}

func TestReadAccess(t *testing.T) {
	r, err := aoClient.NewRef(protectedRef(t, "_adminsdk/go/protected/user2"))
	if err != nil {
		t.Fatal(err)
	}
	var got string
	if err := r.Get(&got); err != nil || got != "test" {
		t.Errorf("Get() = (%q, %v); want = (%q, nil)", got, err, "test")
	}
	if err := r.Set("update"); err == nil {
		t.Errorf("Set() = nil; want = error")
	} else if err.Error() != permDenied {
		t.Errorf("Error = %q; want = %q", err.Error(), permDenied)
	}
}

func TestReadWriteAccess(t *testing.T) {
	r, err := aoClient.NewRef(protectedRef(t, "_adminsdk/go/protected/user1"))
	if err != nil {
		t.Fatal(err)
	}
	var got string
	if err := r.Get(&got); err != nil || got != "test" {
		t.Errorf("Get() = (%q, %v); want = (%q, nil)", got, err, "test")
	}
	if err := r.Set("update"); err != nil {
		t.Errorf("Set() = %v; want = nil", err)
	}
}

func TestQueryAccess(t *testing.T) {
	r, err := aoClient.NewRef("_adminsdk/go/protected")
	if err != nil {
		t.Fatal(err)
	}

	q, err := r.OrderByKey(db.WithLimitToFirst(2))
	if err != nil {
		t.Fatal(err)
	}
	got := make(map[string]interface{})
	if err := q.Get(&got); err == nil {
		t.Errorf("OrderByQuery() = nil; want = error")
	} else if err.Error() != permDenied {
		t.Errorf("Error = %q; want = %q", err.Error(), permDenied)
	}
}

func protectedRef(t *testing.T, p string) string {
	r, err := client.NewRef(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Set("test"); err != nil {
		t.Fatal(err)
	}
	return p
}

type Dinosaur struct {
	Appeared int     `json:"appeared"`
	Height   float64 `json:"height"`
	Length   float64 `json:"length"`
	Order    string  `json:"order"`
	Vanished int     `json:"vanished"`
	Weight   int     `json:"weight"`
	Ratings  Ratings `json:"ratings"`
}

type Ratings struct {
	Pos int `json:"pos"`
}

type User struct {
	Name  string `json:"name"`
	Since int    `json:"since"`
}