import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { DataTable } from "./DataTable.js";

type Row = { id: string; name: string };

describe("DataTable", () => {
  it("renders headers and rows", () => {
    render(
      <DataTable<Row>
        columns={[{ key: "name", header: "Name", render: (r) => r.name }]}
        rows={[
          { id: "1", name: "Alice" },
          { id: "2", name: "Bob" },
        ]}
        getRowKey={(r) => r.id}
      />,
    );
    expect(screen.getByText("Name")).toBeInTheDocument();
    expect(screen.getByText("Alice")).toBeInTheDocument();
    expect(screen.getByText("Bob")).toBeInTheDocument();
  });

  it("invokes onRowClick on click and Enter/Space when provided", () => {
    const onRowClick = vi.fn();
    render(
      <DataTable<Row>
        columns={[{ key: "name", header: "Name", render: (r) => r.name }]}
        rows={[{ id: "1", name: "Alice" }]}
        getRowKey={(r) => r.id}
        onRowClick={onRowClick}
      />,
    );
    const row = screen.getByText("Alice").closest("tr") as HTMLElement;
    expect(row).toHaveAttribute("tabIndex", "0");
    fireEvent.click(row);
    expect(onRowClick).toHaveBeenCalledWith({ id: "1", name: "Alice" });
    fireEvent.keyDown(row, { key: "Enter" });
    expect(onRowClick).toHaveBeenCalledTimes(2);
  });
});
