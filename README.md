# zf-jwxt

A command-line client for ZhengFang (正方) `jwglxt` teaching-management
deployments. It logs in with a password, queries the JSON endpoints a browser
would, and writes what the server answered as plain files.

It knows the protocol and nothing about any particular school: the base URL,
the proxy and the credentials are inputs.

## Build

```sh
go build -trimpath -ldflags='-s -w' -o bin/zf-jwxt ./cmd/zf-jwxt
go test ./...
```

## Capture every class timetable of a term

```sh
export ZF_USERNAME=... ZF_PASSWORD=...
zf-jwxt timetable -base http://jwglxt.example.edu.cn -out data            # current term
zf-jwxt timetable -base http://jwglxt.example.edu.cn -out data -term 2026-2027-1
zf-jwxt timetable ... -proxy socks5://127.0.0.1:1080                     # through a tunnel
```

Output, one directory per term:

```text
data/2026-2027-1/
  term.json          year, term number, observation time
  periods.json       the daily period clock
  catalog.json       every class timetable the term offers
  classes/<id>.json  one class timetable, as answered
```

A class file that exists is not fetched again, so rerunning the command
resumes an interrupted capture. The command prints a JSON summary and exits
non-zero if any class failed. Account and contact fields echoed by the server
(`userModel`, phone numbers, e-mail, …) are dropped before writing.

## Login

The password is RSA-encrypted with the key the server publishes, as the login
page does. A rejected login is reported and **never retried**: ZhengFang locks
an account after a few consecutive failures. An expired session during a
capture triggers a new login, at most five times per run; transport errors
are retried with back-off.

## Scope

Use it with your own account, within the access that account already has.

MIT licence, see [LICENSE](LICENSE).
