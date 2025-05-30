Use testify for unit tests in Go.

In tests, don't use global string variables for test data. Instead, write down
the strings inline in the test cases even if it duplicates some data. It makes
it easier to read each test case.

When writing tests, consider using a single func(t \*testing.T) with multiple
t.Run subtests. Use human-readable names for the subtests names so that it's
easy to spot missing test cases. Example:

```go
func TestCopyKey(t *testing.T) {
    t.Run("happy case", func(t *testing.T) {
    })
    t.Run("show an error when missing source key", func(t *testing.T) {
    })
}
```

Even better, use the following kinda-table-driven approach:

```go
type TestCopyKey_tc struct {
    givenInput string
    expectErr  string
}
func TestCopyKey_run(text TestCopyKey_tc) func(t *testing.T) {
    return func(t *testing.T) {
        // Test logic here
        if text.expectErr != "" {
            assert.EqualError(t, text.expectErr, err)
            return
        }
        assert.NoError(t, err)
    }
}
```

Then, the t.Run subtests can be called like this:

```go
func TestCopyKey(t *testing.T) {
    t.Run("happy case", TestCopyKey_run(TestCopyKey_tc{
        givenInput: "some input",
        expectErr:  "",
    }))
    t.Run("show an error when missing source key", TestCopyKey_run(TestCopyKey_tc{
        givenInput: "missing input",
        expectErr:  "source key not found",
    }))
}
```
