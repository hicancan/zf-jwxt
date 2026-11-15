# zf-jwxt

A command-line client for ZhengFang (正方) teaching-management systems, both generations:

| Directory | System | Protocol |
| --- | --- | --- |
| `jwglxt/` | the current one (`jwglxt`) | JSON over UTF-8, RSA-encrypted password, jqGrid paging |
| `jwxt/` | the classic one (`jwxt`) | ASP.NET WebForms, GB18030, `__VIEWSTATE` postbacks, image captcha |

It asks what a browser would ask and prints what the server answered, every field as it came; nothing is renamed or
left out. It knows the protocols and nothing about any particular school: the base URL, the proxy and the account are
inputs. Which questions to ask, how often and where to keep the answers belong to whoever calls it.

## Build

```sh
go build -trimpath -ldflags='-s -w' -o bin/zf-jwxt ./cmd/zf-jwxt
go test ./...
```

## Signing in

- **Password**: `ZF_USERNAME` and `ZF_PASSWORD`. The current system takes the RSA-encrypted password as its login page
  does; the classic one also needs its captcha read, by a model file the `captcha` package reads (`-captcha-model`) or a command of your own
  (`-captcha-cmd`, image on stdin, text on stdout).
- **A session from elsewhere**: `-cookie 'JSESSIONID=…'`, or `-cookie-cmd` running a command that prints cookies, such
  as [cas-sso](https://github.com/hicancan/cas-sso) for schools that sign in through a unified portal.

A rejected login is reported and never retried: ZhengFang locks an account after a few consecutive failures.

## The current system

```sh
zf-jwxt jwglxt term            -base http://jwglxt.example.edu.cn
zf-jwxt jwglxt class-schedules -base … -term 2026-2027-1 -all
zf-jwxt jwglxt free-rooms      -base … -week 12 -weekday 1 -period 1
zf-jwxt jwglxt raw             -base … -path '/xtgl/comm_cxZydmPaged.html' -all
```

Named commands: `term`, `dict -kind …`, `profile`, `curriculum`, `exec-plan`, `schedule`, `class-schedules`,
`class-timetable`, `periods`, `weekly-schedule`, `rooms`, `free-rooms`, `grades`, `credit-stats`, `exams`,
`unscheduled-exams`, `enrollments`, `selected-courses`, `textbooks`, `evaluations`. `raw` asks any endpoint; `-param
key=value` adds request fields, `-all` follows every page.

**Batch**: many requests on one session. Each line on stdin is a request, each line on stdout its answer, tagged with
the request's `id`:

```sh
zf-jwxt jwglxt batch -base … -workers 2 -delay 300ms < requests.jsonl > answers.jsonl
```

```json
{"id": "B240402", "command": "class-timetable", "term": "2026-2027-1", "params": {"bh_id": "B240402"}}
{"id": "B240402", "ok": true, "result": {"kbList": […]}}
```

A request carries the same fields as the flags (`command`, `term`, `kind`, `path`, `limit`, `page`, `all`, `week`,
`weekday`, `period`, `params`). An expired session is signed in again, at most five times per batch, when the batch
signed in with a password; dropped connections are retried with back-off.

## The classic system

```sh
zf-jwxt jwxt curriculum -base http://jwxt.example.edu.cn -grade 2024 -college 14 -major 1401 -all
zf-jwxt jwxt page       -base … -path xxjsjy.aspx -post -param xiaoq=1
```

Named commands: `curriculum`, `grades`, `level-exams`, `makeup-exams`, `schedule`, `class-schedule`,
`teacher-schedule`, `time-schedule`, `rooms`, `textbooks`, `profile`, `competitions`, `thesis-topics`,
`teaching-plans`. `page` reads any page: its form controls, options and every table, after a postback when asked.

## Scope

Use it with your own account, within the access that account already has.

MIT licence, see [LICENSE](LICENSE).
