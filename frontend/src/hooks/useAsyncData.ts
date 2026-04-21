import { DependencyList, useCallback, useEffect, useState } from "react";

type AsyncState<T> = {
  loading: boolean;
  data: T | null;
  error: string | null;
};

export function useAsyncData<T>(loader: () => Promise<T>, deps: DependencyList = []): AsyncState<T> & { reload: () => void } {
  const [nonce, setNonce] = useState(0);
  const [state, setState] = useState<AsyncState<T>>({
    loading: true,
    data: null,
    error: null
  });

  const reload = useCallback(() => setNonce((value) => value + 1), []);
  useEffect(() => {
    let active = true;
    setState((prev) => ({ ...prev, loading: true, error: null }));

    loader()
      .then((result) => {
        if (!active) {
          return;
        }
        setState({ loading: false, data: result, error: null });
      })
      .catch((err: unknown) => {
        if (!active) {
          return;
        }
        const message = err instanceof Error ? err.message : "Onbekende fout";
        setState({ loading: false, data: null, error: message });
      });

    return () => {
      active = false;
    };
  }, [loader, nonce, ...deps]);

  return { ...state, reload };
}
