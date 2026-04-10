"use client";

import {
  createContext,
  useCallback,
  useContext,
  useState,
} from "react";
import { useSearchParams, usePathname } from "next/navigation";

type PeriodContextValue = {
  period: string;
  setPeriod: (p: string) => void;
};

const PeriodContext = createContext<PeriodContextValue>({
  period: "7d",
  setPeriod: () => {},
});

export function PeriodProvider({ children }: { children: React.ReactNode }) {
  const searchParams = useSearchParams();
  const pathname = usePathname();
  const initial = searchParams.get("period") || "7d";
  const [period, setPeriodState] = useState(initial);

  const setPeriod = useCallback(
    (newPeriod: string) => {
      setPeriodState(newPeriod);
      const params = new URLSearchParams(searchParams.toString());
      params.set("period", newPeriod);
      window.history.replaceState(null, "", `${pathname}?${params.toString()}`);
    },
    [pathname, searchParams]
  );

  return (
    <PeriodContext.Provider value={{ period, setPeriod }}>
      {children}
    </PeriodContext.Provider>
  );
}

export function usePeriod() {
  return useContext(PeriodContext);
}
