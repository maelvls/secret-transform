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
