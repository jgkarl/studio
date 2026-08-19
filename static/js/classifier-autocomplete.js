// Type-ahead replacement for a classifier-bound <select>, no dependency — matches the typed text
// against the <datalist> options (case-insensitive); an unmatched, non-empty value is created on
// the spot via POST /settings/classifiers/{type}/find-or-create and then selected. Markup contract
// (see internal/web/ui.templ's ClassifierAutocomplete): a [data-classifier-autocomplete] wrapper
// carrying data-classifier-type/data-value-key, a hidden [data-ca-value] input (the field actually
// submitted with the form), a text [data-ca-text] input, and its <datalist data-ca-options>.
document.addEventListener("DOMContentLoaded", () => {
  document.querySelectorAll("[data-classifier-autocomplete]").forEach((root) => {
    const type = root.dataset.classifierType;
    const valueKey = root.dataset.valueKey;
    const valueInput = root.querySelector("[data-ca-value]");
    const textInput = root.querySelector("[data-ca-text]");
    const datalist = root.querySelector("[data-ca-options]");

    function findOption(title) {
      const lower = title.toLowerCase();
      for (const opt of datalist.querySelectorAll("option")) {
        if (opt.value.toLowerCase() === lower) return opt;
      }
      return null;
    }

    function select(opt) {
      valueInput.value = valueKey === "code" ? opt.dataset.code : opt.dataset.id;
      textInput.value = opt.value;
    }

    function addOption(id, code, title) {
      const opt = document.createElement("option");
      opt.value = title;
      opt.dataset.id = id;
      opt.dataset.code = code;
      datalist.appendChild(opt);
      return opt;
    }

    textInput.addEventListener("blur", () => {
      const title = textInput.value.trim();
      if (!title) {
        valueInput.value = "";
        return;
      }

      const existing = findOption(title);
      if (existing) {
        select(existing);
        return;
      }

      fetch(`/settings/classifiers/${type}/find-or-create`, {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-Requested-With": "fetch" },
        body: JSON.stringify({ title }),
      })
        .then((res) => (res.ok ? res.json() : Promise.reject(res)))
        .then((created) => select(addOption(created.id, created.code, created.title)))
        .catch(() => {
          /* leave the typed text as-is; the field will fail required-validation on submit
             if nothing got resolved, prompting the user to retry rather than silently losing input */
        });
    });
  });
});
