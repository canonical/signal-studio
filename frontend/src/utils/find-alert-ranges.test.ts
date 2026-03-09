import { describe, it, expect } from "vitest";
import { findAlertRanges } from "./findAlertRanges";

describe("findAlertRanges", () => {
  it("finds a single alert entry", () => {
    const text = `groups:
  - name: test
    rules:
      - alert: HighErrors
        expr: rate(errors[5m]) > 0.1`;
    const ranges = findAlertRanges(text);
    expect(ranges).toHaveLength(1);
    expect(ranges[0]?.name).toBe("HighErrors");
    expect(ranges[0]?.startLine).toBe(4); // 1-based
    expect(ranges[0]?.endLine).toBe(5);
  });

  it("finds multiple alert entries", () => {
    const text = `groups:
  - name: test
    rules:
      - alert: AlertA
        expr: up == 0
      - alert: AlertB
        expr: rate(errors[5m]) > 0.1
        for: 5m`;
    const ranges = findAlertRanges(text);
    expect(ranges).toHaveLength(2);
    expect(ranges[0]?.name).toBe("AlertA");
    expect(ranges[1]?.name).toBe("AlertB");
  });

  it("returns empty for text with no alerts", () => {
    const text = `groups:
  - name: test
    rules:
      - record: some_metric
        expr: sum(rate(x[5m]))`;
    const ranges = findAlertRanges(text);
    expect(ranges).toHaveLength(0);
  });

  it("handles multi-line alert blocks correctly", () => {
    const text = `groups:
  - name: test
    rules:
      - alert: BigAlert
        expr: up == 0
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: host is down`;
    const ranges = findAlertRanges(text);
    expect(ranges).toHaveLength(1);
    expect(ranges[0]?.startLine).toBe(4);
    expect(ranges[0]?.endLine).toBe(10);
  });

  it("handles blank lines within alert blocks", () => {
    const text = `      - alert: Test
        expr: up == 0

        for: 5m`;
    const ranges = findAlertRanges(text);
    expect(ranges).toHaveLength(1);
    expect(ranges[0]?.endLine).toBe(4);
  });
});
