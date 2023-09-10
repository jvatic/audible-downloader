bufseekio
---------

bufseekio provies buffered I/O with io.Seeker interface.

### ReadSeeker

```go
// cache block size   : 128KBytes
// cache block history: 4
r := bufseekio.NewReadSeeker(file, 128 * 1024, 4)
```
