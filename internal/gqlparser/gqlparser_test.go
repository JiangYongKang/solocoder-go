package gqlparser

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

const testSDL = `
type User {
  id: ID!
  name: String!
  email: String
  age: Int
  posts: [Post]
}

type Post {
  id: ID!
  title: String!
  content: String
  author: User
  tags: [String]
}

type Query {
  user(id: ID!): User
  users: [User]
  post(id: ID!): Post
  posts: [Post]
  search(query: String!, limit: Int = 10): [Post]
}

type Mutation {
  createUser(name: String!, email: String): User
  createPost(title: String!, content: String, authorId: ID!): Post
}

schema {
  query: Query
  mutation: Mutation
}

scalar DateTime
`

func setupTestSchema(t *testing.T) *Schema {
	t.Helper()
	s := NewSchema()
	err := s.ParseSDL(testSDL)
	if err != nil {
		t.Fatalf("failed to parse SDL: %v", err)
	}
	return s
}

func TestSchema_ParseSDL(t *testing.T) {
	s := NewSchema()
	err := s.ParseSDL(testSDL)
	if err != nil {
		t.Fatalf("ParseSDL failed: %v", err)
	}

	types := s.GetAllTypes()
	expectedTypes := []string{"User", "Post", "Query", "Mutation", "DateTime", "Int", "Float", "String", "Boolean", "ID"}
	for _, name := range expectedTypes {
		if _, ok := types[name]; !ok {
			t.Errorf("expected type %q to be registered", name)
		}
	}
}

func TestSchema_ParseSDL_InvalidSyntax(t *testing.T) {
	s := NewSchema()
	err := s.ParseSDL("type {")
	if err == nil {
		t.Error("expected error for invalid SDL")
	}
}

func TestSchema_GetType(t *testing.T) {
	s := setupTestSchema(t)

	userType, ok := s.GetType("User")
	if !ok {
		t.Fatal("expected User type to exist")
	}
	if userType.Name != "User" {
		t.Errorf("expected type name User, got %s", userType.Name)
	}
	if len(userType.Fields) != 5 {
		t.Errorf("expected User to have 5 fields, got %d", len(userType.Fields))
	}

	_, ok = s.GetType("NonExistent")
	if ok {
		t.Error("expected non-existent type to return false")
	}
}

func TestSchema_GetBuiltinScalars(t *testing.T) {
	s := NewSchema()

	scalars := []string{"Int", "Float", "String", "Boolean", "ID"}
	for _, name := range scalars {
		tp, ok := s.GetType(name)
		if !ok {
			t.Errorf("expected builtin scalar %s to exist", name)
			continue
		}
		if tp.Kind != TypeKindScalar {
			t.Errorf("expected %s to be scalar kind, got %v", name, tp.Kind)
		}
		if !tp.IsBuiltin {
			t.Errorf("expected %s to be marked as builtin", name)
		}
	}
}

func TestSchema_QueryAndMutationTypes(t *testing.T) {
	s := setupTestSchema(t)

	queryType := s.GetQueryType()
	if queryType == nil {
		t.Fatal("expected query type to be set")
	}
	if queryType.Name != "Query" {
		t.Errorf("expected query type name Query, got %s", queryType.Name)
	}

	mutationType := s.GetMutationType()
	if mutationType == nil {
		t.Fatal("expected mutation type to be set")
	}
	if mutationType.Name != "Mutation" {
		t.Errorf("expected mutation type name Mutation, got %s", mutationType.Name)
	}
}

func TestSchema_RegisterType_Duplicate(t *testing.T) {
	s := NewSchema()
	t1 := &Type{Kind: TypeKindObject, Name: "Test", Fields: map[string]*Field{}}
	err := s.RegisterType(t1)
	if err != nil {
		t.Fatalf("first register should succeed: %v", err)
	}

	t2 := &Type{Kind: TypeKindObject, Name: "Test", Fields: map[string]*Field{}}
	err = s.RegisterType(t2)
	if err != ErrTypeAlreadyExists {
		t.Errorf("expected ErrTypeAlreadyExists, got %v", err)
	}
}

func TestSchema_FieldDefinitions(t *testing.T) {
	s := setupTestSchema(t)

	userType, _ := s.GetType("User")

	idField, ok := userType.Fields["id"]
	if !ok {
		t.Fatal("expected id field on User")
	}
	if !idField.Type.IsNonNull() {
		t.Error("expected id field to be non-null")
	}
	unwrapped := idField.Type.Unwrap()
	if unwrapped.Name != "ID" {
		t.Errorf("expected id field type ID, got %s", unwrapped.Name)
	}

	nameField, ok := userType.Fields["name"]
	if !ok {
		t.Fatal("expected name field on User")
	}
	if !nameField.Type.IsNonNull() {
		t.Error("expected name field to be non-null")
	}

	postsField, ok := userType.Fields["posts"]
	if !ok {
		t.Fatal("expected posts field on User")
	}
	if !postsField.Type.IsList() {
		t.Error("expected posts field to be list type")
	}
}

func TestSchema_RegisterResolver(t *testing.T) {
	s := setupTestSchema(t)

	called := false
	resolver := func(parent interface{}, args map[string]interface{}) (interface{}, error) {
		called = true
		return "test", nil
	}

	err := s.RegisterResolver("Query", "user", resolver)
	if err != nil {
		t.Fatalf("RegisterResolver failed: %v", err)
	}

	gotResolver, ok := s.GetResolver("Query", "user")
	if !ok {
		t.Fatal("expected resolver to be found")
	}

	_, _ = gotResolver(nil, nil)
	if !called {
		t.Error("expected registered resolver to be called")
	}
}

func TestSchema_RegisterResolver_TypeNotFound(t *testing.T) {
	s := NewSchema()
	err := s.RegisterResolver("NonExistent", "field", func(p interface{}, a map[string]interface{}) (interface{}, error) {
		return nil, nil
	})
	if err != ErrTypeNotFound {
		t.Errorf("expected ErrTypeNotFound, got %v", err)
	}
}

func TestSchema_RegisterResolver_FieldNotFound(t *testing.T) {
	s := setupTestSchema(t)
	err := s.RegisterResolver("User", "nonExistent", func(p interface{}, a map[string]interface{}) (interface{}, error) {
		return nil, nil
	})
	if err != ErrFieldNotFound {
		t.Errorf("expected ErrFieldNotFound, got %v", err)
	}
}

func TestSchema_RegisterResolver_Overwrite(t *testing.T) {
	s := setupTestSchema(t)

	first := func(parent interface{}, args map[string]interface{}) (interface{}, error) {
		return "first", nil
	}
	second := func(parent interface{}, args map[string]interface{}) (interface{}, error) {
		return "second", nil
	}

	_ = s.RegisterResolver("Query", "user", first)
	_ = s.RegisterResolver("Query", "user", second)

	resolver, _ := s.GetResolver("Query", "user")
	result, _ := resolver(nil, nil)
	if result != "second" {
		t.Errorf("expected second resolver to overwrite, got %v", result)
	}
}

func TestParseQuery_SimpleQuery(t *testing.T) {
	query := `
		query GetUser {
			user(id: "1") {
				id
				name
			}
		}
	`

	doc, err := ParseQuery(query)
	if err != nil {
		t.Fatalf("ParseQuery failed: %v", err)
	}

	if len(doc.Operations) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(doc.Operations))
	}

	op := doc.Operations[0]
	if op.Type != OperationQuery {
		t.Errorf("expected query operation, got %v", op.Type)
	}
	if op.Name != "GetUser" {
		t.Errorf("expected operation name GetUser, got %s", op.Name)
	}
	if len(op.SelectionSet) != 1 {
		t.Fatalf("expected 1 selection, got %d", len(op.SelectionSet))
	}

	field, ok := (*op.SelectionSet[0]).(*FieldSelection)
	if !ok {
		t.Fatal("expected FieldSelection")
	}
	if field.Name != "user" {
		t.Errorf("expected field name user, got %s", field.Name)
	}
	if len(field.Args) != 1 {
		t.Errorf("expected 1 arg, got %d", len(field.Args))
	}
	if field.Args["id"] != "1" {
		t.Errorf("expected id arg to be '1', got %v", field.Args["id"])
	}
}

func TestParseQuery_ShorthandQuery(t *testing.T) {
	query := `{
		user(id: "1") {
			name
		}
	}`

	doc, err := ParseQuery(query)
	if err != nil {
		t.Fatalf("ParseQuery failed: %v", err)
	}

	if len(doc.Operations) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(doc.Operations))
	}
	if doc.Operations[0].Type != OperationQuery {
		t.Error("expected query operation type")
	}
}

func TestParseQuery_Mutation(t *testing.T) {
	query := `
		mutation CreateUser {
			createUser(name: "Alice", email: "alice@example.com") {
				id
				name
			}
		}
	`

	doc, err := ParseQuery(query)
	if err != nil {
		t.Fatalf("ParseQuery failed: %v", err)
	}

	op := doc.Operations[0]
	if op.Type != OperationMutation {
		t.Errorf("expected mutation operation, got %v", op.Type)
	}
	if op.Name != "CreateUser" {
		t.Errorf("expected operation name CreateUser, got %s", op.Name)
	}
}

func TestParseQuery_WithAliases(t *testing.T) {
	query := `{
		user1: user(id: "1") {
			fullName: name
		}
		user2: user(id: "2") {
			fullName: name
		}
	}`

	doc, err := ParseQuery(query)
	if err != nil {
		t.Fatalf("ParseQuery failed: %v", err)
	}

	op := doc.Operations[0]
	if len(op.SelectionSet) != 2 {
		t.Fatalf("expected 2 selections, got %d", len(op.SelectionSet))
	}

	field1, ok := (*op.SelectionSet[0]).(*FieldSelection)
	if !ok {
		t.Fatal("expected FieldSelection")
	}
	if field1.Alias != "user1" {
		t.Errorf("expected alias user1, got %s", field1.Alias)
	}
	if field1.Name != "user" {
		t.Errorf("expected field name user, got %s", field1.Name)
	}
}

func TestParseQuery_NestedSelections(t *testing.T) {
	query := `{
		user(id: "1") {
			name
			posts {
				title
				author {
					name
				}
			}
		}
	}`

	doc, err := ParseQuery(query)
	if err != nil {
		t.Fatalf("ParseQuery failed: %v", err)
	}

	op := doc.Operations[0]
	userField := (*op.SelectionSet[0]).(*FieldSelection)
	if len(userField.SelectionSet) != 2 {
		t.Fatalf("expected 2 sub-selections, got %d", len(userField.SelectionSet))
	}

	var postsField *FieldSelection
	for _, sel := range userField.SelectionSet {
		if f, ok := (*sel).(*FieldSelection); ok && f.Name == "posts" {
			postsField = f
			break
		}
	}
	if postsField == nil {
		t.Fatal("expected posts field")
	}
	if len(postsField.SelectionSet) != 2 {
		t.Fatalf("expected 2 selections in posts, got %d", len(postsField.SelectionSet))
	}
}

func TestParseQuery_WithVariables(t *testing.T) {
	query := `
		query GetUser($id: ID!, $limit: Int = 10) {
			user(id: $id) {
				name
				posts(limit: $limit) {
					title
				}
			}
		}
	`

	doc, err := ParseQuery(query)
	if err != nil {
		t.Fatalf("ParseQuery failed: %v", err)
	}

	op := doc.Operations[0]
	if len(op.VariableDefs) != 2 {
		t.Fatalf("expected 2 variable defs, got %d", len(op.VariableDefs))
	}

	idVar := op.VariableDefs[0]
	if idVar.Name != "id" {
		t.Errorf("expected first var name id, got %s", idVar.Name)
	}
	if !idVar.Type.IsNonNull() {
		t.Error("expected id var to be non-null")
	}

	limitVar := op.VariableDefs[1]
	if limitVar.Name != "limit" {
		t.Errorf("expected second var name limit, got %s", limitVar.Name)
	}
	if limitVar.DefaultValue != 10 {
		t.Errorf("expected limit default value 10, got %v", limitVar.DefaultValue)
	}
}

func TestParseQuery_InvalidSyntax(t *testing.T) {
	queries := []string{
		"",
		"query { user(",
		"{ user(id:) }",
		"mutation { }",
	}

	for i, q := range queries {
		_, err := ParseQuery(q)
		if err == nil {
			t.Errorf("query %d: expected error for invalid query", i)
		}
	}
}

func TestValidator_ValidQuery(t *testing.T) {
	s := setupTestSchema(t)
	v := NewValidator()

	query := `{
		user(id: "1") {
			id
			name
		}
	}`

	doc, _ := ParseQuery(query)
	errs := v.Validate(s, doc)
	if len(errs) > 0 {
		t.Errorf("expected no validation errors, got %d: %v", len(errs), errs)
	}
}

func TestValidator_FieldNotFound(t *testing.T) {
	s := setupTestSchema(t)
	v := NewValidator()

	query := `{
		user(id: "1") {
			id
			nonExistentField
		}
	}`

	doc, _ := ParseQuery(query)
	errs := v.Validate(s, doc)
	if len(errs) == 0 {
		t.Error("expected validation error for unknown field")
	}
}

func TestValidator_TopLevelFieldNotFound(t *testing.T) {
	s := setupTestSchema(t)
	v := NewValidator()

	query := `{
		nonExistentField(id: "1") {
			id
		}
	}`

	doc, _ := ParseQuery(query)
	errs := v.Validate(s, doc)
	if len(errs) == 0 {
		t.Error("expected validation error for unknown top-level field")
	}
}

func TestValidator_RequiredArgumentMissing(t *testing.T) {
	s := setupTestSchema(t)
	v := NewValidator()

	query := `{
		user {
			id
		}
	}`

	doc, _ := ParseQuery(query)
	errs := v.Validate(s, doc)
	if len(errs) == 0 {
		t.Error("expected validation error for missing required argument")
	}
}

func TestValidator_DepthLimitExceeded(t *testing.T) {
	s := setupTestSchema(t)
	v := NewValidatorWithMaxDepth(2)

	query := `{
		user(id: "1") {
			posts {
				author {
					posts {
						title
					}
				}
			}
		}
	}`

	doc, _ := ParseQuery(query)
	errs := v.Validate(s, doc)
	if len(errs) == 0 {
		t.Error("expected validation error for depth limit exceeded")
	}
}

func TestValidator_UnknownArgument(t *testing.T) {
	s := setupTestSchema(t)
	v := NewValidator()

	query := `{
		user(id: "1", unknown: "value") {
			id
		}
	}`

	doc, _ := ParseQuery(query)
	errs := v.Validate(s, doc)
	if len(errs) == 0 {
		t.Error("expected validation error for unknown argument")
	}
}

func TestValidator_InvalidArgType(t *testing.T) {
	s := setupTestSchema(t)
	v := NewValidator()

	query := `{
		search(query: 123) {
			title
		}
	}`

	doc, _ := ParseQuery(query)
	errs := v.Validate(s, doc)
	if len(errs) == 0 {
		t.Error("expected validation error for invalid argument type")
	}
}

func TestValidator_MutationWithoutMutationType(t *testing.T) {
	s := NewSchema()
	sdl := `
		type Query {
			hello: String
		}
		schema { query: Query }
	`
	_ = s.ParseSDL(sdl)
	v := NewValidator()

	query := `mutation { test }`
	doc, _ := ParseQuery(query)
	errs := v.Validate(s, doc)
	if len(errs) == 0 {
		t.Error("expected validation error for mutation without mutation type")
	}
}

func TestValidator_InlineFragment(t *testing.T) {
	s := setupTestSchema(t)
	v := NewValidator()

	query := `{
		user(id: "1") {
			... on User {
				name
			}
		}
	}`

	doc, _ := ParseQuery(query)
	errs := v.Validate(s, doc)
	if len(errs) > 0 {
		t.Errorf("expected no errors for valid inline fragment, got %v", errs)
	}
}

func TestValidator_InlineFragmentUnknownType(t *testing.T) {
	s := setupTestSchema(t)
	v := NewValidator()

	query := `{
		user(id: "1") {
			... on NonExistent {
				name
			}
		}
	}`

	doc, _ := ParseQuery(query)
	errs := v.Validate(s, doc)
	if len(errs) == 0 {
		t.Error("expected validation error for inline fragment with unknown type")
	}
}

func TestDataLoader_Load(t *testing.T) {
	var callCount int
	var receivedKeys []interface{}

	dl := NewDataLoader(func(keys []interface{}) ([]interface{}, error) {
		callCount++
		receivedKeys = append(receivedKeys, keys...)
		results := make([]interface{}, len(keys))
		for i, k := range keys {
			results[i] = fmt.Sprintf("loaded:%v", k)
		}
		return results, nil
	})

	var result interface{}
	var err error
	done := make(chan struct{})
	go func() {
		result, err = dl.Load("key1")
		close(done)
	}()

	time.Sleep(10 * time.Millisecond)
	_ = dl.Flush()
	<-done

	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if result != "loaded:key1" {
		t.Errorf("expected loaded:key1, got %v", result)
	}
}

func TestDataLoader_BatchLoading(t *testing.T) {
	var callCount int
	var mu sync.Mutex
	var lastKeys []interface{}

	dl := NewDataLoader(func(keys []interface{}) ([]interface{}, error) {
		mu.Lock()
		callCount++
		lastKeys = make([]interface{}, len(keys))
		copy(lastKeys, keys)
		mu.Unlock()

		results := make([]interface{}, len(keys))
		for i, k := range keys {
			results[i] = fmt.Sprintf("val:%v", k)
		}
		return results, nil
	})

	var wg sync.WaitGroup
	results := make([]interface{}, 3)
	errs := make([]error, 3)
	var readyWg sync.WaitGroup

	for i, key := range []interface{}{"a", "b", "c"} {
		wg.Add(1)
		readyWg.Add(1)
		go func(idx int, k interface{}) {
			defer wg.Done()
			readyWg.Done()
			val, err := dl.Load(k)
			results[idx] = val
			errs[idx] = err
		}(i, key)
	}

	readyWg.Wait()
	time.Sleep(10 * time.Millisecond)
	_ = dl.Flush()
	wg.Wait()

	if callCount != 1 {
		t.Errorf("expected 1 batch call, got %d", callCount)
	}

	expected := []string{"val:a", "val:b", "val:c"}
	for i, exp := range expected {
		if errs[i] != nil {
			t.Errorf("result %d: unexpected error %v", i, errs[i])
		}
		if results[i] != exp {
			t.Errorf("result %d: expected %s, got %v", i, exp, results[i])
		}
	}
}

func TestDataLoader_LoadMany(t *testing.T) {
	dl := NewDataLoader(func(keys []interface{}) ([]interface{}, error) {
		results := make([]interface{}, len(keys))
		for i, k := range keys {
			results[i] = fmt.Sprintf("result:%v", k)
		}
		return results, nil
	})

	var vals []interface{}
	var errs []error
	done := make(chan struct{})
	go func() {
		vals, errs = dl.LoadMany([]interface{}{"x", "y", "z"})
		close(done)
	}()

	time.Sleep(10 * time.Millisecond)
	_ = dl.Flush()
	<-done

	for i, err := range errs {
		if err != nil {
			t.Errorf("LoadMany index %d error: %v", i, err)
		}
	}
	expected := []string{"result:x", "result:y", "result:z"}
	for i, exp := range expected {
		if vals[i] != exp {
			t.Errorf("LoadMany index %d: expected %s, got %v", i, exp, vals[i])
		}
	}
}

func TestDataLoader_Clear(t *testing.T) {
	dl := NewDataLoader(func(keys []interface{}) ([]interface{}, error) {
		results := make([]interface{}, len(keys))
		for i, k := range keys {
			results[i] = k
		}
		return results, nil
	})

	go func() { dl.Load("key1") }()
	go func() { dl.Load("key2") }()
	time.Sleep(10 * time.Millisecond)

	dl.Clear("key1")

	if len(dl.pending) != 1 {
		t.Errorf("expected 1 pending request after clear, got %d", len(dl.pending))
	}
	if dl.pending[0].key != "key2" {
		t.Errorf("expected remaining key to be key2, got %v", dl.pending[0].key)
	}
	dl.ClearAll()
}

func TestDataLoader_ClearAll(t *testing.T) {
	dl := NewDataLoader(func(keys []interface{}) ([]interface{}, error) {
		return make([]interface{}, len(keys)), nil
	})

	go func() { dl.Load("key1") }()
	go func() { dl.Load("key2") }()
	go func() { dl.Load("key3") }()
	time.Sleep(10 * time.Millisecond)

	dl.ClearAll()

	if len(dl.pending) != 0 {
		t.Errorf("expected 0 pending requests after clear all, got %d", len(dl.pending))
	}
}

func setupTestExecutor(t *testing.T) (*Executor, map[string]*DataLoader) {
	t.Helper()
	s := setupTestSchema(t)

	users := map[string]map[string]interface{}{
		"1": {"id": "1", "name": "Alice", "email": "alice@example.com", "age": 30},
		"2": {"id": "2", "name": "Bob", "email": "bob@example.com", "age": 25},
	}
	posts := map[string]map[string]interface{}{
		"101": {"id": "101", "title": "First Post", "content": "Hello World", "authorId": "1", "tags": []string{"go", "graphql"}},
		"102": {"id": "102", "title": "Second Post", "content": "Second", "authorId": "2", "tags": []string{"test"}},
		"103": {"id": "103", "title": "Third Post", "content": "Third", "authorId": "1", "tags": []string{"go"}},
	}
	userPosts := map[string][]string{
		"1": {"101", "103"},
		"2": {"102"},
	}

	_ = s.RegisterResolver("Query", "user", func(parent interface{}, args map[string]interface{}) (interface{}, error) {
		id := fmt.Sprintf("%v", args["id"])
		return users[id], nil
	})

	_ = s.RegisterResolver("Query", "users", func(parent interface{}, args map[string]interface{}) (interface{}, error) {
		result := make([]map[string]interface{}, 0, len(users))
		for _, u := range users {
			result = append(result, u)
		}
		return result, nil
	})

	_ = s.RegisterResolver("Query", "post", func(parent interface{}, args map[string]interface{}) (interface{}, error) {
		id := fmt.Sprintf("%v", args["id"])
		return posts[id], nil
	})

	_ = s.RegisterResolver("Query", "posts", func(parent interface{}, args map[string]interface{}) (interface{}, error) {
		result := make([]map[string]interface{}, 0, len(posts))
		for _, p := range posts {
			result = append(result, p)
		}
		return result, nil
	})

	_ = s.RegisterResolver("User", "posts", func(parent interface{}, args map[string]interface{}) (interface{}, error) {
		p, ok := parent.(map[string]interface{})
		if !ok {
			return nil, nil
		}
		userId := fmt.Sprintf("%v", p["id"])
		postIds := userPosts[userId]
		result := make([]map[string]interface{}, 0, len(postIds))
		for _, pid := range postIds {
			result = append(result, posts[pid])
		}
		return result, nil
	})

	_ = s.RegisterResolver("Post", "author", func(parent interface{}, args map[string]interface{}) (interface{}, error) {
		p, ok := parent.(map[string]interface{})
		if !ok {
			return nil, nil
		}
		authorId := fmt.Sprintf("%v", p["authorId"])
		return users[authorId], nil
	})

	userDL := NewDataLoader(func(keys []interface{}) ([]interface{}, error) {
		results := make([]interface{}, len(keys))
		for i, k := range keys {
			id := fmt.Sprintf("%v", k)
			results[i] = users[id]
		}
		return results, nil
	})

	postDL := NewDataLoader(func(keys []interface{}) ([]interface{}, error) {
		results := make([]interface{}, len(keys))
		for i, k := range keys {
			id := fmt.Sprintf("%v", k)
			results[i] = posts[id]
		}
		return results, nil
	})

	dls := map[string]*DataLoader{
		"user": userDL,
		"post": postDL,
	}

	exec := NewExecutor(s)
	return exec, dls
}

func TestExecutor_SimpleQuery(t *testing.T) {
	exec, dls := setupTestExecutor(t)

	query := `{
		user(id: "1") {
			id
			name
			email
			age
		}
	}`

	result := exec.Execute(query, nil, dls)
	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	userData, ok := result.Data["user"].(map[string]interface{})
	if !ok {
		t.Fatal("expected user data to be map")
	}

	if userData["id"] != "1" {
		t.Errorf("expected id 1, got %v", userData["id"])
	}
	if userData["name"] != "Alice" {
		t.Errorf("expected name Alice, got %v", userData["name"])
	}
	if userData["email"] != "alice@example.com" {
		t.Errorf("expected email alice@example.com, got %v", userData["email"])
	}
	if userData["age"] != 30 {
		t.Errorf("expected age 30, got %v", userData["age"])
	}
}

func TestExecutor_ListQuery(t *testing.T) {
	exec, dls := setupTestExecutor(t)

	query := `{
		users {
			id
			name
		}
	}`

	result := exec.Execute(query, nil, dls)
	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	users, ok := result.Data["users"].([]interface{})
	if !ok {
		t.Fatalf("expected users list, got %T", result.Data["users"])
	}
	if len(users) != 2 {
		t.Errorf("expected 2 users, got %d", len(users))
	}
}

func TestExecutor_NestedQuery(t *testing.T) {
	exec, dls := setupTestExecutor(t)

	query := `{
		user(id: "1") {
			name
			posts {
				title
				tags
			}
		}
	}`

	result := exec.Execute(query, nil, dls)
	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	userData, ok := result.Data["user"].(map[string]interface{})
	if !ok {
		t.Fatal("expected user data")
	}

	posts, ok := userData["posts"].([]interface{})
	if !ok {
		t.Fatal("expected posts list")
	}
	if len(posts) != 2 {
		t.Errorf("expected 2 posts for user 1, got %d", len(posts))
	}
}

func TestExecutor_NestedCircular(t *testing.T) {
	exec, dls := setupTestExecutor(t)

	query := `{
		post(id: "101") {
			title
			author {
				name
			}
		}
	}`

	result := exec.Execute(query, nil, dls)
	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	postData, ok := result.Data["post"].(map[string]interface{})
	if !ok {
		t.Fatal("expected post data")
	}

	author, ok := postData["author"].(map[string]interface{})
	if !ok {
		t.Fatal("expected author data")
	}
	if author["name"] != "Alice" {
		t.Errorf("expected author name Alice, got %v", author["name"])
	}
}

func TestExecutor_QueryWithAlias(t *testing.T) {
	exec, dls := setupTestExecutor(t)

	query := `{
		alice: user(id: "1") {
			fullName: name
		}
		bob: user(id: "2") {
			fullName: name
		}
	}`

	result := exec.Execute(query, nil, dls)
	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	alice, ok := result.Data["alice"].(map[string]interface{})
	if !ok {
		t.Fatal("expected alice data")
	}
	if alice["fullName"] != "Alice" {
		t.Errorf("expected fullName Alice, got %v", alice["fullName"])
	}

	bob, ok := result.Data["bob"].(map[string]interface{})
	if !ok {
		t.Fatal("expected bob data")
	}
	if bob["fullName"] != "Bob" {
		t.Errorf("expected fullName Bob, got %v", bob["fullName"])
	}
}

func TestExecutor_QueryWithVariables(t *testing.T) {
	exec, dls := setupTestExecutor(t)

	query := `
		query GetUser($userId: ID!) {
			user(id: $userId) {
				id
				name
			}
		}
	`

	variables := map[string]interface{}{
		"userId": "2",
	}

	result := exec.Execute(query, variables, dls)
	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	userData, ok := result.Data["user"].(map[string]interface{})
	if !ok {
		t.Fatal("expected user data")
	}
	if userData["name"] != "Bob" {
		t.Errorf("expected user Bob, got %v", userData["name"])
	}
}

func TestExecutor_ValidationError(t *testing.T) {
	exec, dls := setupTestExecutor(t)

	query := `{
		user(id: "1") {
			nonExistent
		}
	}`

	result := exec.Execute(query, nil, dls)
	if len(result.Errors) == 0 {
		t.Error("expected validation errors")
	}
}

func TestExecutor_ParseError(t *testing.T) {
	exec, dls := setupTestExecutor(t)

	result := exec.Execute("invalid query{", nil, dls)
	if len(result.Errors) == 0 {
		t.Error("expected parse errors")
	}
}

func TestType_Unwrap(t *testing.T) {
	inner := &Type{Kind: TypeKindScalar, Name: "String"}
	list := &Type{Kind: TypeKindList, OfType: inner}
	nonNull := &Type{Kind: TypeKindNonNull, OfType: list}

	unwrapped := nonNull.Unwrap()
	if unwrapped.Kind != TypeKindScalar {
		t.Errorf("expected scalar after unwrap, got %v", unwrapped.Kind)
	}
	if unwrapped.Name != "String" {
		t.Errorf("expected String after unwrap, got %s", unwrapped.Name)
	}
}

func TestType_IsList(t *testing.T) {
	inner := &Type{Kind: TypeKindScalar, Name: "Int"}
	list := &Type{Kind: TypeKindList, OfType: inner}
	nonNullList := &Type{Kind: TypeKindNonNull, OfType: list}
	nonNullScalar := &Type{Kind: TypeKindNonNull, OfType: inner}

	if !list.IsList() {
		t.Error("expected list to be list type")
	}
	if !nonNullList.IsList() {
		t.Error("expected non-null list to be list type")
	}
	if nonNullScalar.IsList() {
		t.Error("expected non-null scalar to not be list type")
	}
	if inner.IsList() {
		t.Error("expected scalar to not be list type")
	}
}

func TestType_IsNonNull(t *testing.T) {
	scalar := &Type{Kind: TypeKindScalar, Name: "String"}
	nonNull := &Type{Kind: TypeKindNonNull, OfType: scalar}

	if scalar.IsNonNull() {
		t.Error("expected scalar to not be non-null")
	}
	if !nonNull.IsNonNull() {
		t.Error("expected wrapped to be non-null")
	}
}

func TestSchema_SetQueryType_NotFound(t *testing.T) {
	s := NewSchema()
	err := s.SetQueryType("NonExistent")
	if err != ErrTypeNotFound {
		t.Errorf("expected ErrTypeNotFound, got %v", err)
	}
}

func TestSchema_SetMutationType_NotFound(t *testing.T) {
	s := NewSchema()
	err := s.SetMutationType("NonExistent")
	if err != ErrTypeNotFound {
		t.Errorf("expected ErrTypeNotFound, got %v", err)
	}
}

func TestValidationError_Error(t *testing.T) {
	e1 := &ValidationError{Message: "test error"}
	if e1.Error() != "test error" {
		t.Errorf("expected 'test error', got %q", e1.Error())
	}

	e2 := &ValidationError{Path: "user.name", Message: "required"}
	if e2.Error() != "user.name: required" {
		t.Errorf("expected 'user.name: required', got %q", e2.Error())
	}
}

func TestNewValidationError(t *testing.T) {
	e := NewValidationError("query.user", "field %s not found", "name")
	if e.Path != "query.user" {
		t.Errorf("expected path query.user, got %s", e.Path)
	}
	if e.Message != "field name not found" {
		t.Errorf("expected message, got %s", e.Message)
	}
}

func TestExecutor_DefaultVariableValue(t *testing.T) {
	s := setupTestSchema(t)

	_ = s.RegisterResolver("Query", "search", func(parent interface{}, args map[string]interface{}) (interface{}, error) {
		limit, _ := args["limit"].(int)
		if limit != 10 {
			t.Errorf("expected default limit 10, got %v", limit)
		}
		return []map[string]interface{}{
			{"id": "1", "title": "Test"},
		}, nil
	})

	exec := NewExecutor(s)

	query := `
		query Search($q: String!, $limit: Int = 10) {
			search(query: $q, limit: $limit) {
				title
			}
		}
	`
	variables := map[string]interface{}{"q": "test"}
	result := exec.Execute(query, variables, nil)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}
}

func TestDataLoader_EmptyFlush(t *testing.T) {
	dl := NewDataLoader(func(keys []interface{}) ([]interface{}, error) {
		t.Error("batch function should not be called for empty flush")
		return nil, nil
	})

	err := dl.Flush()
	if err != nil {
		t.Errorf("unexpected error on empty flush: %v", err)
	}
}

func TestExecutor_MultipleFields(t *testing.T) {
	exec, dls := setupTestExecutor(t)

	query := `{
		user(id: "1") {
			name
		}
		post(id: "101") {
			title
		}
	}`

	result := exec.Execute(query, nil, dls)
	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	userData, ok := result.Data["user"].(map[string]interface{})
	if !ok {
		t.Fatal("expected user data")
	}
	if userData["name"] != "Alice" {
		t.Errorf("expected Alice, got %v", userData["name"])
	}

	postData, ok := result.Data["post"].(map[string]interface{})
	if !ok {
		t.Fatal("expected post data")
	}
	if postData["title"] != "First Post" {
		t.Errorf("expected First Post, got %v", postData["title"])
	}
}

func TestSchema_ParseSDL_WithComments(t *testing.T) {
	sdlWithComments := `
		# This is a comment
		type TestType {
			# Field comment
			field1: String!
			field2: Int
		}
	`
	s := NewSchema()
	err := s.ParseSDL(sdlWithComments)
	if err != nil {
		t.Fatalf("ParseSDL with comments failed: %v", err)
	}

	tp, ok := s.GetType("TestType")
	if !ok {
		t.Fatal("expected TestType to be registered")
	}
	if len(tp.Fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(tp.Fields))
	}
}

func TestValidator_DefaultMaxDepth(t *testing.T) {
	v := NewValidator()
	if v.MaxDepth != 10 {
		t.Errorf("expected default max depth 10, got %d", v.MaxDepth)
	}
}

func TestParseQuery_BooleanAndNullValues(t *testing.T) {
	query := `{
		test(flag: true, other: false, nothing: null) {
			id
		}
	}`

	s := NewSchema()
	sdl := `
		type Query {
			test(flag: Boolean, other: Boolean, nothing: String): TestType
		}
		type TestType { id: ID! }
		schema { query: Query }
	`
	_ = s.ParseSDL(sdl)

	doc, err := ParseQuery(query)
	if err != nil {
		t.Fatalf("ParseQuery failed: %v", err)
	}

	field := (*doc.Operations[0].SelectionSet[0]).(*FieldSelection)
	if field.Args["flag"] != true {
		t.Errorf("expected flag true, got %v", field.Args["flag"])
	}
	if field.Args["other"] != false {
		t.Errorf("expected other false, got %v", field.Args["other"])
	}
	if field.Args["nothing"] != nil {
		t.Errorf("expected nothing nil, got %v", field.Args["nothing"])
	}
}

func TestParseQuery_ListValue(t *testing.T) {
	query := `{
		test(tags: ["go", "graphql", "test"]) {
			id
		}
	}`

	s := NewSchema()
	sdl := `
		type Query {
			test(tags: [String]): TestType
		}
		type TestType { id: ID! }
		schema { query: Query }
	`
	_ = s.ParseSDL(sdl)

	doc, err := ParseQuery(query)
	if err != nil {
		t.Fatalf("ParseQuery failed: %v", err)
	}

	field := (*doc.Operations[0].SelectionSet[0]).(*FieldSelection)
	tags, ok := field.Args["tags"].([]interface{})
	if !ok {
		t.Fatalf("expected tags to be list, got %T", field.Args["tags"])
	}
	if len(tags) != 3 {
		t.Errorf("expected 3 tags, got %d", len(tags))
	}
}

func TestParseQuery_ObjectValue(t *testing.T) {
	query := `{
		test(filter: {name: "test", active: true}) {
			id
		}
	}`

	s := NewSchema()
	sdl := `
		type Query {
			test(filter: FilterInput): TestType
		}
		type TestType { id: ID! }
		input FilterInput { name: String, active: Boolean }
		schema { query: Query }
	`
	_ = s.ParseSDL(sdl)

	doc, err := ParseQuery(query)
	if err != nil {
		t.Fatalf("ParseQuery failed: %v", err)
	}

	field := (*doc.Operations[0].SelectionSet[0]).(*FieldSelection)
	filter, ok := field.Args["filter"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected filter to be object, got %T", field.Args["filter"])
	}
	if filter["name"] != "test" {
		t.Errorf("expected filter.name test, got %v", filter["name"])
	}
	if filter["active"] != true {
		t.Errorf("expected filter.active true, got %v", filter["active"])
	}
}

func TestParseQuery_FloatValues(t *testing.T) {
	query := `{
		test(price: 9.99, discount: -0.5) {
			id
		}
	}`

	s := NewSchema()
	sdl := `
		type Query {
			test(price: Float, discount: Float): TestType
		}
		type TestType { id: ID! }
		schema { query: Query }
	`
	_ = s.ParseSDL(sdl)

	doc, err := ParseQuery(query)
	if err != nil {
		t.Fatalf("ParseQuery failed: %v", err)
	}

	field := (*doc.Operations[0].SelectionSet[0]).(*FieldSelection)
	price, ok := field.Args["price"].(float64)
	if !ok {
		t.Fatalf("expected price to be float64, got %T", field.Args["price"])
	}
	if price < 9.98 || price > 10.0 {
		t.Errorf("expected price ~9.99, got %v", price)
	}
}

func TestExecutor_ResolverReturnsError(t *testing.T) {
	s := setupTestSchema(t)

	_ = s.RegisterResolver("Query", "user", func(parent interface{}, args map[string]interface{}) (interface{}, error) {
		return nil, fmt.Errorf("database connection failed")
	})

	exec := NewExecutor(s)

	query := `{
		user(id: "1") {
			name
		}
	}`

	result := exec.Execute(query, nil, nil)
	if len(result.Errors) == 0 {
		t.Error("expected errors from resolver")
	}
}

func TestSchema_GetResolver_NotFound(t *testing.T) {
	s := setupTestSchema(t)

	_, ok := s.GetResolver("Query", "user")
	if ok {
		t.Error("expected no resolver to be found")
	}

	_, ok = s.GetResolver("NonExistent", "field")
	if ok {
		t.Error("expected no resolver for non-existent type")
	}
}

func TestSchema_ParseSDL_ScalarDefinition(t *testing.T) {
	sdl := `
		scalar CustomScalar
		type Query {
			test: CustomScalar
		}
		schema { query: Query }
	`
	s := NewSchema()
	err := s.ParseSDL(sdl)
	if err != nil {
		t.Fatalf("ParseSDL failed: %v", err)
	}

	tp, ok := s.GetType("CustomScalar")
	if !ok {
		t.Fatal("expected CustomScalar to exist")
	}
	if tp.Kind != TypeKindScalar {
		t.Errorf("expected scalar kind, got %v", tp.Kind)
	}
}

func TestExecutor_ResolverReturnsStruct(t *testing.T) {
	type TestUser struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	s := NewSchema()
	sdl := `
		type User {
			id: ID!
			name: String!
			age: Int
		}
		type Query {
			user(id: ID!): User
		}
		schema { query: Query }
	`
	_ = s.ParseSDL(sdl)

	_ = s.RegisterResolver("Query", "user", func(parent interface{}, args map[string]interface{}) (interface{}, error) {
		return &TestUser{
			ID:   "1",
			Name: "StructUser",
			Age:  42,
		}, nil
	})

	exec := NewExecutor(s)
	query := `{ user(id: "1") { id name age } }`
	result := exec.Execute(query, nil, nil)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	userData, ok := result.Data["user"].(map[string]interface{})
	if !ok {
		t.Fatal("expected user data to be map")
	}
	if userData["name"] != "StructUser" {
		t.Errorf("expected StructUser, got %v", userData["name"])
	}
	if userData["age"] != 42 {
		t.Errorf("expected 42, got %v", userData["age"])
	}
}
