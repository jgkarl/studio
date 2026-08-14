// TipTap wiring for the Reporter module — genuine TipTap (core + starter-kit + image), loaded
// live via the esm.sh ESM CDN through the page's <script type="importmap"> rather than a
// downloaded/vendored bundle: TipTap's npm packages aren't a single flat browser file the way
// OpenSeadragon's UMD build is, and this project deliberately has no JS bundler to produce one.
// This is the honest trade-off — the real library, zero build step, at the cost of a runtime
// dependency on esm.sh being reachable.
import { Editor } from "@tiptap/core";
import StarterKit from "@tiptap/starter-kit";
import Image from "@tiptap/extension-image";

const TOOLBAR_BUTTONS = [
  { cmd: "undo", label: "Undo ↺", group: 0 },
  { cmd: "redo", label: "Redo ↻", group: 0 },
  { cmd: "toggleBold", label: "B", group: 1, mark: "bold" },
  { cmd: "toggleItalic", label: "I", group: 1, mark: "italic" },
  { cmd: "toggleUnderline", label: "U", group: 1, mark: "underline" },
  { cmd: "toggleStrike", label: "S", group: 1, mark: "strike" },
  { cmd: "toggleCode", label: "Code", group: 1, mark: "code" },
  { cmd: "toggleHeading1", label: "H1", group: 2 },
  { cmd: "toggleHeading2", label: "H2", group: 2 },
  { cmd: "toggleHeading3", label: "H3", group: 2 },
  { cmd: "toggleBulletList", label: "• List", group: 3, mark: "bulletList" },
  { cmd: "toggleOrderedList", label: "1. List", group: 3, mark: "orderedList" },
  { cmd: "toggleBlockquote", label: "Quote", group: 3, mark: "blockquote" },
  { cmd: "toggleCodeBlock", label: "Code block", group: 3, mark: "codeBlock" },
  { cmd: "setHorizontalRule", label: "―", group: 3 },
  { cmd: "link", label: "Link", group: 4 },
  { cmd: "image", label: "Image", group: 5 },
  { cmd: "clear", label: "Clear format", group: 6 },
];

document.addEventListener("DOMContentLoaded", () => {
  const root = document.getElementById("report-editor");
  if (!root) return;

  const reportId = root.dataset.reportId;
  const readOnly = root.dataset.readonly !== undefined;
  const toolbar = document.getElementById("report-editor-toolbar");
  const contentEl = document.getElementById("report-editor-content");

  let initialContent = { type: "doc", content: [] };
  try {
    initialContent = JSON.parse(root.dataset.content || "{}");
  } catch {
    // malformed/empty stored content - start blank rather than fail the whole page
  }

  const editor = new Editor({
    element: contentEl,
    extensions: [StarterKit.configure({ link: { openOnClick: false } }), Image],
    content: initialContent,
    editable: !readOnly,
  });

  if (readOnly) return; // final reports render read-only, no toolbar, no save button at all

  let buttonEls = [];

  function updateActiveStates() {
    buttonEls.forEach(({ el, def }) => {
      if (def.mark) el.classList.toggle("is-active", editor.isActive(def.mark));
    });
  }

  function runCommand(cmd) {
    const chain = editor.chain().focus();
    switch (cmd) {
      case "undo":
        chain.undo().run();
        break;
      case "redo":
        chain.redo().run();
        break;
      case "toggleBold":
        chain.toggleBold().run();
        break;
      case "toggleItalic":
        chain.toggleItalic().run();
        break;
      case "toggleUnderline":
        chain.toggleUnderline().run();
        break;
      case "toggleStrike":
        chain.toggleStrike().run();
        break;
      case "toggleCode":
        chain.toggleCode().run();
        break;
      case "toggleHeading1":
        chain.toggleHeading({ level: 1 }).run();
        break;
      case "toggleHeading2":
        chain.toggleHeading({ level: 2 }).run();
        break;
      case "toggleHeading3":
        chain.toggleHeading({ level: 3 }).run();
        break;
      case "toggleBulletList":
        chain.toggleBulletList().run();
        break;
      case "toggleOrderedList":
        chain.toggleOrderedList().run();
        break;
      case "toggleBlockquote":
        chain.toggleBlockquote().run();
        break;
      case "toggleCodeBlock":
        chain.toggleCodeBlock().run();
        break;
      case "setHorizontalRule":
        chain.setHorizontalRule().run();
        break;
      case "link":
        if (editor.isActive("link")) {
          chain.unsetLink().run();
        } else {
          const url = window.prompt("Link URL:", "https://");
          if (url) chain.extendMarkRange("link").setLink({ href: url }).run();
        }
        break;
      case "clear":
        chain.clearNodes().unsetAllMarks().run();
        break;
      case "image":
        pickAndInsertImage();
        return; // async path, skip the synchronous updateActiveStates below
    }
    updateActiveStates();
  }

  TOOLBAR_BUTTONS.forEach((def) => {
    const btn = document.createElement("button");
    btn.type = "button";
    btn.className = "btn btn-ghost report-editor-btn";
    btn.textContent = def.label;
    btn.title = def.label;
    btn.addEventListener("click", () => runCommand(def.cmd));
    toolbar.appendChild(btn);
    buttonEls.push({ el: btn, def });
  });

  const saveBtn = document.createElement("button");
  saveBtn.type = "button";
  saveBtn.className = "btn btn-primary report-editor-save";
  saveBtn.textContent = "Save";
  saveBtn.addEventListener("click", async () => {
    saveBtn.disabled = true;
    saveBtn.textContent = "Saving…";
    try {
      const res = await fetch(`/reporter/${reportId}/content`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(editor.getJSON()),
      });
      saveBtn.textContent = res.ok ? "Saved" : "Save failed";
    } catch {
      saveBtn.textContent = "Save failed";
    }
    setTimeout(() => {
      saveBtn.textContent = "Save";
      saveBtn.disabled = false;
    }, 1200);
  });
  toolbar.appendChild(saveBtn);

  const imageInput = document.createElement("input");
  imageInput.type = "file";
  imageInput.accept = "image/*";
  imageInput.style.display = "none";
  imageInput.addEventListener("change", async () => {
    const file = imageInput.files && imageInput.files[0];
    imageInput.value = "";
    if (!file) return;
    const fd = new FormData();
    fd.append("file", file);
    const res = await fetch(`/reporter/${reportId}/image`, { method: "POST", body: fd });
    if (!res.ok) return;
    const { url } = await res.json();
    editor.chain().focus().setImage({ src: url }).run();
  });
  document.body.appendChild(imageInput);

  function pickAndInsertImage() {
    imageInput.click();
  }

  editor.on("selectionUpdate", updateActiveStates);
  editor.on("transaction", updateActiveStates);
  updateActiveStates();
});
