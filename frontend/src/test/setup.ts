import "@testing-library/jest-dom/vitest";

// jsdom doesn't implement the <dialog> element's showModal()/close() (used by
// @shared/ui's Modal for its native focus-trap/top-layer behavior) — polyfill just
// enough for it to open/close under Testing Library.
if (typeof HTMLDialogElement !== "undefined") {
  if (!HTMLDialogElement.prototype.showModal) {
    HTMLDialogElement.prototype.showModal = function showModal(
      this: HTMLDialogElement,
    ) {
      this.setAttribute("open", "");
    };
  }
  if (!HTMLDialogElement.prototype.close) {
    HTMLDialogElement.prototype.close = function close(
      this: HTMLDialogElement,
    ) {
      this.removeAttribute("open");
      this.dispatchEvent(new Event("close"));
    };
  }
}
