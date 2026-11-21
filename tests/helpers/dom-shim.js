/**
 * Lightweight zero-dependency DOM shim for Gate 2 Pure Grex JS v1.0.0 verification.
 * Tracks every innerHTML write attempt explicitly via innerHTMLWriteCount.
 */

class ShimTextNode {
  constructor(text, ownerDocument) {
    this.nodeType = 3;
    this.nodeValue = String(text ?? "");
    this.parentNode = null;
    this.ownerDocument = ownerDocument;
  }

  get textContent() {
    return this.nodeValue;
  }

  set textContent(v) {
    this.nodeValue = String(v ?? "");
  }
}

class ShimStyle {
  constructor() {
    this.props = new Map();
  }

  setProperty(name, value) {
    this.props.set(String(name), String(value));
  }

  getPropertyValue(name) {
    return this.props.get(String(name)) || "";
  }
}

class ShimElement {
  constructor(tagName, ownerDocument, counter) {
    this.nodeType = 1;
    this.localName = String(tagName).toLowerCase();
    this.tagName = this.localName.toUpperCase();
    this.ownerDocument = ownerDocument;
    this._counter = counter;
    this.attributes = new Map();
    this.childNodes = [];
    this.parentNode = null;
    this.listeners = new Map();
    this.style = new ShimStyle();
    this._value = "";
  }

  get id() {
    return this.attributes.get("id") || "";
  }

  set id(v) {
    this.attributes.set("id", String(v));
  }

  get className() {
    return this.attributes.get("class") || "";
  }

  set className(v) {
    this.attributes.set("class", String(v));
  }

  get value() {
    if (this.attributes.has("value") && this._value === "") {
      return this.attributes.get("value");
    }
    return this._value;
  }

  set value(v) {
    this._value = String(v ?? "");
  }

  get firstChild() {
    return this.childNodes.length > 0 ? this.childNodes[0] : null;
  }

  get children() {
    return this.childNodes.filter((c) => c.nodeType === 1);
  }

  get innerHTML() {
    return "";
  }

  set innerHTML(val) {
    this._counter.innerHTMLWriteCount += 1;
    throw new Error(`innerHTML write forbidden under Pure Grex JS (attempted "${val}")`);
  }

  get textContent() {
    return this.childNodes.map((c) => c.textContent).join("");
  }

  set textContent(v) {
    for (const c of this.childNodes) {
      c.parentNode = null;
    }
    this.childNodes = [];
    if (v !== "" && v != null) {
      this.appendChild(new ShimTextNode(String(v), this.ownerDocument));
    }
  }

  setAttribute(name, value) {
    this.attributes.set(String(name).toLowerCase(), String(value ?? ""));
  }

  getAttribute(name) {
    const key = String(name).toLowerCase();
    return this.attributes.has(key) ? this.attributes.get(key) : null;
  }

  hasAttribute(name) {
    return this.attributes.has(String(name).toLowerCase());
  }

  removeAttribute(name) {
    this.attributes.delete(String(name).toLowerCase());
  }

  appendChild(child) {
    if (child.parentNode && typeof child.parentNode.removeChild === "function") {
      child.parentNode.removeChild(child);
    }
    child.parentNode = this;
    this.childNodes.push(child);
    return child;
  }

  insertBefore(newNode, refNode) {
    if (!refNode) {
      return this.appendChild(newNode);
    }
    if (newNode.parentNode && typeof newNode.parentNode.removeChild === "function") {
      newNode.parentNode.removeChild(newNode);
    }
    const idx = this.childNodes.indexOf(refNode);
    if (idx === -1) {
      return this.appendChild(newNode);
    }
    newNode.parentNode = this;
    this.childNodes.splice(idx, 0, newNode);
    return newNode;
  }

  removeChild(child) {
    const idx = this.childNodes.indexOf(child);
    if (idx !== -1) {
      this.childNodes.splice(idx, 1);
      child.parentNode = null;
    }
    return child;
  }

  replaceChildren(...nodes) {
    for (const c of this.childNodes) {
      c.parentNode = null;
    }
    this.childNodes = [];
    for (const n of nodes) {
      this.appendChild(n);
    }
  }

  addEventListener(type, fn) {
    const key = String(type);
    if (!this.listeners.has(key)) {
      this.listeners.set(key, new Set());
    }
    this.listeners.get(key).add(fn);
  }

  removeEventListener(type, fn) {
    const set = this.listeners.get(String(type));
    if (set) {
      set.delete(fn);
    }
  }

  async dispatchEvent(ev) {
    const eventObj = ev || { type: "click" };
    if (!eventObj.target) {
      eventObj.target = this;
    }
    if (typeof eventObj.preventDefault !== "function") {
      eventObj.preventDefault = () => {};
    }
    const set = this.listeners.get(String(eventObj.type));
    if (set) {
      for (const fn of Array.from(set)) {
        await fn(eventObj);
      }
    }
  }

  async click() {
    await this.dispatchEvent({ type: "click" });
  }

  focus() {
    if (this.ownerDocument) {
      this.ownerDocument.activeElement = this;
    }
  }

  querySelectorAll(selector) {
    const results = [];
    const parts = String(selector)
      .split(",")
      .map((s) => s.trim())
      .filter(Boolean);

    const walk = (node) => {
      for (const child of node.childNodes) {
        if (child.nodeType === 1) {
          if (parts.some((sel) => matchesSimpleSelector(child, sel))) {
            results.push(child);
          }
          walk(child);
        }
      }
    };
    walk(this);
    return results;
  }

  querySelector(selector) {
    const matches = this.querySelectorAll(selector);
    return matches.length > 0 ? matches[0] : null;
  }
}

function matchesSimpleSelector(el, sel) {
  if (sel === "*") return true;
  if (sel.startsWith("#")) {
    return el.id === sel.slice(1);
  }
  if (sel.startsWith(".")) {
    const cls = (el.getAttribute("class") || "").split(/\s+/);
    return cls.includes(sel.slice(1));
  }
  // Support tag[attr="val"] and [attr], img[alt], label[for="..."]
  const parts = sel.split(/\s+/);
  if (parts.length > 1) {
    const targetSel = parts[parts.length - 1];
    if (!matchesAtom(el, targetSel)) return false;
    let p = el.parentNode;
    for (let i = parts.length - 2; i >= 0; i--) {
      while (p && p.nodeType === 1 && !matchesAtom(p, parts[i])) {
        p = p.parentNode;
      }
      if (!p || p.nodeType !== 1) return false;
      p = p.parentNode;
    }
    return true;
  }
  return matchesAtom(el, sel);
}

function matchesAtom(el, atom) {
  let rem = atom;
  const tagMatch = rem.match(/^([a-z0-9*-]+)/i);
  if (tagMatch) {
    const expectedTag = tagMatch[1].toLowerCase();
    if (expectedTag !== "*" && el.localName !== expectedTag) {
      return false;
    }
    rem = rem.slice(tagMatch[1].length);
  }
  while (rem.length > 0) {
    if (rem.startsWith("#")) {
      const m = rem.match(/^#([a-z0-9_-]+)/i);
      if (!m || el.id !== m[1]) return false;
      rem = rem.slice(m[0].length);
    } else if (rem.startsWith(".")) {
      const m = rem.match(/^\.([a-z0-9_-]+)/i);
      const cls = (el.getAttribute("class") || "").split(/\s+/);
      if (!m || !cls.includes(m[1])) return false;
      rem = rem.slice(m[0].length);
    } else if (rem.startsWith("[")) {
      const m = rem.match(/^\[([a-z0-9_-]+)(?:=["']([^"']*)["'])?\]/i);
      if (!m) return false;
      const val = el.getAttribute(m[1]);
      if (val === null) return false;
      if (m[2] !== undefined && val !== m[2]) return false;
      rem = rem.slice(m[0].length);
    } else {
      return false;
    }
  }
  return true;
}

export function createDomShim(initialTheme = "light") {
  const counter = { innerHTMLWriteCount: 0 };
  const doc = {
    readyState: "complete",
    documentElement: null,
    body: null,
    get innerHTMLWriteCount() {
      return counter.innerHTMLWriteCount;
    },
    createElement(tag) {
      return new ShimElement(tag, doc, counter);
    },
    createElementNS(_ns, tag) {
      return new ShimElement(tag, doc, counter);
    },
    createTextNode(text) {
      return new ShimTextNode(text, doc);
    },
    getElementById(id) {
      return doc.documentElement.querySelector(`#${id}`);
    },
    querySelector(sel) {
      return doc.documentElement.querySelector(sel);
    },
    querySelectorAll(sel) {
      return doc.documentElement.querySelectorAll(sel);
    },
  };
  doc.documentElement = new ShimElement("html", doc, counter);
  doc.documentElement.setAttribute("lang", "en");
  doc.documentElement.setAttribute("data-grex-theme", initialTheme);
  doc.body = new ShimElement("body", doc, counter);
  doc.documentElement.appendChild(doc.body);
  return {
    document: doc,
    get innerHTMLWriteCount() {
      return counter.innerHTMLWriteCount;
    },
  };
}
