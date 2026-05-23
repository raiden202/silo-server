import { describe, it, expect } from "vitest";
import { formatBirthDate, computeAge } from "@/lib/date";

describe("formatBirthDate", () => {
  it("formats a standard date", () => {
    expect(formatBirthDate("1977-04-23")).toBe("Apr 23, 1977");
  });

  it("formats a date with single-digit day", () => {
    expect(formatBirthDate("1990-01-05")).toBe("Jan 5, 1990");
  });

  it("formats a leap year date", () => {
    expect(formatBirthDate("2000-02-29")).toBe("Feb 29, 2000");
  });
});

describe("computeAge", () => {
  it("computes age for a living person", () => {
    const today = new Date();
    const birthYear = today.getFullYear() - 30;
    const birthMonth = String(today.getMonth() + 1).padStart(2, "0");
    const birthDay = String(today.getDate()).padStart(2, "0");
    expect(computeAge(`${birthYear}-${birthMonth}-${birthDay}`)).toBe(30);
  });

  it("subtracts one year if birthday has not occurred yet", () => {
    const today = new Date();
    const futureMonth = today.getMonth() + 2; // 1-2 months from now
    if (futureMonth <= 12) {
      const birthYear = today.getFullYear() - 25;
      const monthStr = String(futureMonth).padStart(2, "0");
      expect(computeAge(`${birthYear}-${monthStr}-15`)).toBe(24);
    }
  });

  it("computes age at death", () => {
    expect(computeAge("1950-06-15", "2020-03-10")).toBe(69);
  });

  it("computes age at death when death is after birthday in same year", () => {
    expect(computeAge("1950-03-10", "2020-06-15")).toBe(70);
  });
});
