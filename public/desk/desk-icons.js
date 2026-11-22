import { Icon } from "../grex/grex.js";

const icons = {
  brand: [
    { tag: "path", attrs: { d: "M12 0.8 22 6.5v11L12 23.2 2 17.5v-11Zm0 4L5.5 8.5v7L12 19.2l6.5-3.7v-4.8H12v3h3.4v.2L12 15.8l-3.5-2V10l3.5-2 3 1.7 1.7-2.1Z", fill: "currentColor", "fill-rule": "evenodd" } },
  ],
  database: [
    { tag: "path", attrs: { d: "M3 5.5a9 3.5 0 1 0 18 0 9 3.5 0 1 0-18 0Z", fill: "currentColor" } },
    { tag: "path", attrs: { d: "M3 8.5c2 1.5 5.3 2.5 9 2.5s7-1 9-2.5V12c0 2-4 3.5-9 3.5S3 14 3 12Zm0 6.5c2 1.5 5.3 2.5 9 2.5s7-1 9-2.5v3.5c0 2-4 3.5-9 3.5s-9-1.5-9-3.5Z", fill: "currentColor" } },
  ],
  addConnection: [
    { tag: "path", attrs: { d: "M12 1a11 11 0 1 0 0 22 11 11 0 0 0 0-22Zm0 4.5a1.25 1.25 0 0 1 1.25 1.25v4h4a1.25 1.25 0 0 1 0 2.5h-4v4a1.25 1.25 0 0 1-2.5 0v-4h-4a1.25 1.25 0 0 1 0-2.5h4v-4A1.25 1.25 0 0 1 12 5.5Z", fill: "currentColor", "fill-rule": "evenodd" } },
  ],
  search: [
    { tag: "circle", attrs: { cx: "10.5", cy: "10.5", r: "6.5", stroke: "currentColor", "stroke-width": "2" } },
    { tag: "path", attrs: { d: "m15.2 15.2 5.3 5.3", stroke: "currentColor", "stroke-width": "2", "stroke-linecap": "round" } },
  ],
  history: [
    { tag: "path", attrs: { d: "M3 5v5h5m-4.7 0A9 9 0 1 1 4.4 17M12 7v5l3 2", stroke: "currentColor", "stroke-width": "2", "stroke-linecap": "round", "stroke-linejoin": "round" } },
  ],
  settingsOutline: [
    { tag: "path", attrs: { d: "M10.25 1.6h3.5l.2 2.5 2.35.95 1.9-1.65 2.45 2.45-1.65 1.9.95 2.35 2.5.2v3.5l-2.5.2-.95 2.35 1.65 1.9-2.45 2.45-1.9-1.65-2.35.95-.2 2.5h-3.5l-.2-2.5-2.35-.95-1.9 1.65-2.45-2.45 1.65-1.9-.95-2.35-2.5-.2v-3.5l2.5-.2.95-2.35-1.65-1.9L5.8 3.4l1.9 1.65 2.35-.95Z", stroke: "currentColor", "stroke-width": "1.45", "stroke-linejoin": "round" } },
    { tag: "circle", attrs: { cx: "12", cy: "12", r: "3.4", stroke: "currentColor", "stroke-width": "1.75" } },
  ],
  settingsSolid: [
    { tag: "path", attrs: { d: "M10 2h4l.8 3.1 2.1 1.2 3.1-.9 2 3.5-2.2 2.2v1.8l2.2 2.2-2 3.5-3.1-.9-2.1 1.2L14 22h-4l-.8-3.1-2.1-1.2-3.1.9-2-3.5 2.2-2.2v-1.8L2 8.9l2-3.5 3.1.9 2.1-1.2Zm2 6.5a3.5 3.5 0 1 0 0 7 3.5 3.5 0 0 0 0-7Z", fill: "currentColor", "fill-rule": "evenodd", "stroke-linejoin": "round" } },
  ],
};

export function deskIcon(doc, name, className = "desk-inline-icon", size = "14") {
  return Icon({ className, props: { size, paths: icons[name] } }, { document: doc }).element;
}
