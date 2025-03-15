Watch a log file and use SSE to stream the changes to a page.

```bash
go build
```

Either pass a file or directory containing log files:

```bash
./logwatcher -file /path/to/file.log
```

or

```bash
./logwatcher -dir /path/to/logfiles/
```

In this case, the most recently modified log file will be used.
