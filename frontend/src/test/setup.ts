import "@testing-library/jest-dom/vitest";

// jsdom does not implement <dialog> autofocus/showModal/close (the Forge Modal
// atom relies on the native top-layer <dialog> API — see @shared/ui's
// Modal.tsx). Polyfill the bits used so unit tests can render an open Modal
// without a real browser; Storybook (Chromium) already exercises the real
// native behavior.
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
