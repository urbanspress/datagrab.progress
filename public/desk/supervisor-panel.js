/**
 * Pure Grex JS v1.0.0 Local Grisp (grab.exe) Multi-Worker Supervisor Panel.
 * Preserves exact ADR-025c base-10 wire strings for port, PID, reload count, and log counts.
 */
import { ForEach, signal } from "../grex/grex.js";

export function createSupervisorPanelNode(store, actions, doc = globalThis.document) {
  const newWorkerId = signal("worker-1");
  const newWorkerSubdir = signal(".");
  const newWorkerGrab = signal("src/desk.grab");
  const supervisorStatus = signal(
    "Up to 8 local loopback Grisp workers (127.0.0.1:8765-8772)."
  );

  async function handleStartWorker() {
    try {
      supervisorStatus.set("Starting Grisp worker...");
      const started = await actions.startWorker({
        id: String(newWorkerId.peek() || "").trim(),
        name: String(newWorkerId.peek() || "").trim(),
        subdir: String(newWorkerSubdir.peek() || ".").trim(),
        grab_file: String(newWorkerGrab.peek() || "src/desk.grab").trim(),
      });
      supervisorStatus.set(
        `Worker ${started.id} online at ${started.url} (PID ${started.pid}).`
      );
    } catch (err) {
      supervisorStatus.set(err && err.message ? err.message : String(err));
    }
  }

  return {
    type: "Card",
    tag: "section",
    id: "desk-grisp-supervisor",
    attrs: { id: "desk-grisp-supervisor" },
    className: "desk-card",
    children: [
      {
        type: "Heading",
        props: { level: 3, text: "Grisp Multi-Worker Supervisor" },
      },
      {
        type: "Text",
        tag: "p",
        id: "desk-grisp-status",
        attrs: { id: "desk-grisp-status" },
        className: "desk-muted",
        props: { text: supervisorStatus },
      },
      {
        type: "Form",
        id: "desk-grisp-start-form",
        attrs: { id: "desk-grisp-start-form" },
        props: {
          onSubmit: async () => {
            await handleStartWorker();
          },
        },
        children: [
          {
            type: "Input",
            id: "grisp-worker-id",
            attrs: { id: "grisp-worker-id" },
            props: {
              name: "id",
              label: "Worker ID",
              value: newWorkerId,
              required: true,
            },
          },
          {
            type: "Input",
            id: "grisp-worker-subdir",
            attrs: { id: "grisp-worker-subdir" },
            props: {
              name: "subdir",
              label: "Project Subdir",
              value: newWorkerSubdir,
            },
          },
          {
            type: "Input",
            id: "grisp-worker-grab",
            attrs: { id: "grisp-worker-grab" },
            props: {
              name: "grab_file",
              label: "Entry .grab File",
              value: newWorkerGrab,
              required: true,
            },
          },
          {
            type: "Stack",
            tag: "section",
            className: "desk-badge-row",
            children: [
              {
                type: "Button",
                id: "desk-grisp-start-btn",
                attrs: { id: "desk-grisp-start-btn" },
                className: "desk-btn-primary",
                props: { type: "submit", text: "Start Grisp Worker" },
              },
              {
                type: "Button",
                id: "desk-grisp-refresh-btn",
                attrs: { id: "desk-grisp-refresh-btn" },
                props: {
                  type: "button",
                  text: "Refresh Workers",
                  onClick: () =>
                    actions
                      .refreshWorkers()
                      .catch((e) => supervisorStatus.set(e.message)),
                },
              },
            ],
          },
        ],
      },
      ForEach(
        {
          items: store.workers,
          key: (w) => w.id,
          parent: {
            type: "Section",
            tag: "section",
            className: "desk-list",
            props: { label: "Running Grisp Workers" },
          },
          empty: () => ({
            type: "Text",
            tag: "p",
            className: "desk-muted",
            props: { text: "No local Grisp workers started yet." },
          }),
          render: (workerSig) => {
            const id = workerSig.peek().id;
            return {
              type: "Card",
              tag: "article",
              className: "desk-card",
              children: [
                {
                  type: "Text",
                  tag: "strong",
                  props: {
                    text: () => {
                      const w = workerSig.get();
                      return `${w.id} — ${w.url} (PID ${w.pid})`;
                    },
                  },
                },
                {
                  type: "Text",
                  tag: "span",
                  className: "desk-muted",
                  props: {
                    text: () => {
                      const w = workerSig.get();
                      return `Status: ${w.status} | Health: ${w.health_status} | Ready: ${w.ready_status} | Reloads: ${w.reload_count}`;
                    },
                  },
                },
                {
                  type: "Stack",
                  tag: "section",
                  className: "desk-badge-row",
                  children: [
                    {
                      type: "Button",
                      props: {
                        type: "button",
                        text: "Atomic Reload",
                        onClick: () =>
                          actions
                            .reloadWorker(id)
                            .then((w) =>
                              supervisorStatus.set(
                                `Reloaded ${w.id} cleanly (PID ${w.pid}).`
                              )
                            )
                            .catch((e) => supervisorStatus.set(e.message)),
                      },
                    },
                    {
                      type: "Button",
                      props: {
                        type: "button",
                        text: "View Logs",
                        onClick: () =>
                          actions
                            .loadWorkerLogs(id)
                            .catch((e) => supervisorStatus.set(e.message)),
                      },
                    },
                    {
                      type: "Button",
                      props: {
                        type: "button",
                        text: "Stop",
                        onClick: () =>
                          actions
                            .stopWorker(id)
                            .then(() =>
                              supervisorStatus.set(`Stopped worker ${id}.`)
                            )
                            .catch((e) => supervisorStatus.set(e.message)),
                      },
                    },
                  ],
                },
              ],
            };
          },
        },
        { document: doc }
      ),
      {
        type: "Text",
        tag: "code",
        id: "desk-worker-logs-output",
        attrs: { id: "desk-worker-logs-output" },
        className: "desk-code-block",
        props: { text: store.activeLogsText },
      },
    ],
  };
}
