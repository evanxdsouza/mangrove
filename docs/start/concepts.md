# Concepts

A handful of nouns cover everything in Mangrove.

```
Workspace
 └─ Project            (optionally linked to one GitHub repo)
     └─ Deployment     (production, staging, or a PR preview)
         ├─ Service    (one container, or one per compose service)
         │   └─ Environment variables
         └─ Runs       (deploy history: every build and rollback)
```

## Workspace

A group of projects, usually one per team or purpose. Every install has a
default workspace that can't be deleted. Access is granted per workspace:
someone can be an admin in one and a viewer in another. See
[Workspaces and roles](../teams/workspaces-and-roles.md).

The **station switcher** at the top of the sidebar picks which workspace
the Projects page shows.

## Project

A group of deployments that belong together, such as a web app, its
worker and its database. A project can be linked to one GitHub repository;
its deployments then build from that repo, each from its own branch.

## Deployment

The thing you deploy and open. It has:

- **A build strategy**: how it becomes something runnable. One of
  Dockerfile, nixpacks, compose, image or static. See
  [Builds](../deploy/builds.md).
- **An environment**: production, staging or preview. Staging and preview
  deployments are copies of a production one, tracking another branch or a
  pull request. See [Environments and variables](../run/environments.md).
- **A status**: pending (never deployed), building, running, stopped,
  sleeping or failed.
- **Settings** for access control, idle sleep, self-healing, domains,
  auto-deploy and replicas, all on its **Overview** tab.

## Service

One container inside a deployment. Most deployments have one. A compose
deployment has one per compose service. Each service has its own:

- internal port, and a public host port unless it is **internal only**;
- CPU and memory limits;
- health check path;
- environment variables.

## Run

One attempt at deploying: a build, a redeploy, a rollback or a scale
change. Runs are listed newest first on the **History** tab, with who or
what triggered them (manual, push, redeploy, rollback, scale, promote,
auto-retry), the commit, and how long they took. The one currently serving
traffic is marked **current**. Rolling back means deploying an older
successful run again. See [Rollbacks](../deploy/rollbacks.md).

## Template

A recipe for a common app, such as WordPress plus MySQL, that creates and
wires up one or more deployments in a project. See
[Templates](../run/templates.md).

## Owner and member

Every account is either an **owner**, who runs the box (users, ports,
backups, storage), or a **member**, who only sees the workspaces they've
been added to. The first account is always an owner.

## The box

Everything runs on one machine. Mangrove's memory budget for apps is
limited by `MANGROVE_DEPLOYMENT_MEMORY_CEILING_MB`. A deploy that would go
over it is refused before it starts. See
[Scaling and resources](../deploy/scaling.md).
