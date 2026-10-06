import { describe, expect, it } from "vitest";
import {
  combine32,
  decodeClock,
  encodeClock,
  formatVolume,
  split32,
} from "./lib";

describe("register value helpers", () => {
  it("combines and splits 32-bit values in register order", () => {
    expect(combine32(0x8776, 0x6554)).toBe(0x87766554);
    expect(split32(0x87766554)).toEqual([0x8776, 0x6554]);
  });

  it("converts flow counts using the documented output pulse equivalent", () => {
    expect(formatVolume(8575, 1)).toBe("8,575 L · 8.575 m³");
    expect(formatVolume(1234, 10)).toBe("12,340 L · 12.34 m³");
    expect(formatVolume(8575, undefined)).toBeNull();
  });

  it("encodes and decodes the manual clock byte layout", () => {
    const registers = encodeClock("2017-05-11T08:14:50");
    expect(registers).toEqual([0x1105, 0x0b08, 0x0e32]);
    expect(decodeClock(registers)).toBe("2017-05-11 08:14:50");
  });

  it("rejects invalid clock dates and oversized counters", () => {
    expect(() => encodeClock("")).toThrow();
    expect(() => split32(0x100000000)).toThrow();
    expect(decodeClock([0x1113, 0x0b08, 0x0e32])).toBeNull();
  });
});
