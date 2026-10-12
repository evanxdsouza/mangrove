# Notifications

Mangrove tells you about deploys in three places.

## Email

Set two variables, and Mangrove emails you when a deployment deploys
successfully:

```sh
MANGROVE_RESEND_API_KEY=re_...
MANGROVE_NOTIFY_EMAIL=you@example.com
```

Emails go through [Resend](https://resend.com). They're sent from
`Mangrove <onboarding@resend.dev>`, which needs no domain setup. Set
`MANGROVE_RESEND_FROM` to use your own verified sender.

You also get an email when a deployment becomes publicly reachable for the
first time.

There's one recipient per install, and only successful deploys are
emailed. A failed deploy shows in the dashboard and, for GitHub deploys, as
a failed commit status.

Every email attempt, sent or failed, is listed under **Admin →
Notifications**.

## GitHub commit statuses

For deploys triggered by a push or a PR, Mangrove posts a status to the
commit: pending while it builds, then success or failure. It shows up on
the commit and the pull request like a CI check.

## Pull request comments

With [PR previews](environments.md#pr-previews) on, each PR gets one bot
comment that's edited as the preview builds, showing "building", then the
preview's address or the error.

## Milestones

The dashboard shows a one-time message for your first successful deploy
and your first rollback in each browser. That's all; there's no
notification feed.
