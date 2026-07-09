(function () {
  function updateButton(button, input) {
    if (!button || !input) return;
    const visible = input.type === "text";
    const showLabel = button.getAttribute("data-show-label") || "Show password";
    const hideLabel = button.getAttribute("data-hide-label") || "Hide password";
    const icon = button.querySelector("i");
    if (icon) {
      icon.className = visible ? "fa-regular fa-eye-slash" : "fa-regular fa-eye";
    }
    button.setAttribute("aria-label", visible ? hideLabel : showLabel);
    button.setAttribute("title", visible ? hideLabel : showLabel);
    button.setAttribute("aria-pressed", visible ? "true" : "false");
  }

  function bindPasswordToggle(wrapper) {
    if (!wrapper || wrapper.dataset.passwordToggleBound === "true") return;
    const input = wrapper.querySelector("input[data-password-toggle]");
    const button = wrapper.querySelector("button[data-password-toggle-btn]");
    if (!input || !button) return;
    wrapper.dataset.passwordToggleBound = "true";
    updateButton(button, input);
    button.addEventListener("click", () => {
      input.type = input.type === "password" ? "text" : "password";
      updateButton(button, input);
      input.focus({ preventScroll: true });
      const length = input.value.length;
      try {
        input.setSelectionRange(length, length);
      } catch (_error) {}
    });
  }

  function init(root = document) {
    root.querySelectorAll(".password-input-wrap").forEach(bindPasswordToggle);
  }

  window.UnyDeskPasswordToggle = { init };
  document.addEventListener("DOMContentLoaded", () => init(document));
})();
