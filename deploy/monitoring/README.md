# Alert catalogue

`alerts.csv` lists what must raise an alert, from which signal, and how far that signal exists today. It is a catalogue, not
a monitoring system: nothing here sends a page. Production needs a metrics and alerting backend, an on-call rota and a way
to deliver pages (none exists yet; README, "Production infrastructure").

`signal_status` is one of:

- `exists`: the service already emits the signal. It still needs a rule in the alerting backend.
- `needs-wiring`: the check exists as a command or function but nothing runs it on a schedule or turns its result into an alert.
- `missing`: the signal does not exist yet.

Every alert has a runbook in `docs/runbooks/README.md` under a heading with the same id. `python3 -I scripts/check-monitoring.py`
checks that, and that every register id it cites exists in `docs/regulatory/register.csv`.

Joint Standard 2 of 2024, paragraph 7.3, asks for monitoring that is continuous, with protected and correlated logs, alerts that
someone actively watches, and escalation. This catalogue is the list of what to watch. Watching it is still to be done.
