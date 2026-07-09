import { render, screen, fireEvent } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { Dropzone } from "./Dropzone.js";

function makeFileList(files: File[]): FileList {
  const fileList = {
    length: files.length,
    item: (index: number) => files[index] ?? null,
  } as FileList & { [index: number]: File };
  files.forEach((file, index) => {
    fileList[index] = file;
  });
  return fileList;
}

describe("Dropzone", () => {
  it("calls onFilesSelected when files are dropped", () => {
    const onFilesSelected = vi.fn();
    const { container } = render(<Dropzone onFilesSelected={onFilesSelected} />);

    const dropzone = screen.getByTestId("dropzone");

    const file = new File(["hello"], "contract.pdf", {
      type: "application/pdf",
    });
    const files = makeFileList([file]);

    fireEvent.dragOver(dropzone);
    expect(dropzone).toHaveAttribute("data-dragging", "true");

    fireEvent.drop(dropzone, {
      dataTransfer: { files },
    });

    expect(onFilesSelected).toHaveBeenCalledOnce();
    expect(onFilesSelected.mock.calls[0]?.[0]).toBe(files);
    expect(container.querySelector('input[type="file"]')).not.toBeNull();
  });

  it("opens the hidden file input when clicked", () => {
    const clickSpy = vi.spyOn(HTMLInputElement.prototype, "click");

    render(<Dropzone onFilesSelected={vi.fn()} />);
    fireEvent.click(screen.getByRole("button", { name: /^browse files$/i }));

    expect(clickSpy).toHaveBeenCalledOnce();
    clickSpy.mockRestore();
  });
});
