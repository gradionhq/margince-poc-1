import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
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
});
