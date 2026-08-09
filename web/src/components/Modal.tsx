import { createContext, useContext, useEffect, useRef, useState, type ReactNode } from "react";
import { createSpring, prefersReducedMotion, type SpringHandle } from "../lib/spring";

// Lets a Cancel/Close button inside a modal's own content trigger the same
// spring-out exit as clicking the backdrop or pressing Escape, instead of
// calling the onClose prop directly and skipping the animation.
const ModalCloseContext = createContext<() => void>(() => {});
export function useModalClose(): () => void {
  return useContext(ModalCloseContext);
}

export function Modal({ title, onClose, children }: { title: string; onClose: () => void; children: ReactNode }) {
  const [progress, setProgress] = useState(0);
  const springRef = useRef<SpringHandle | null>(null);
  const closingRef = useRef(false);
  const onCloseRef = useRef(onClose);
  onCloseRef.current = onClose;

  useEffect(() => {
    if (prefersReducedMotion()) {
      setProgress(1);
      return;
    }
    const spring = createSpring(0, {
      damping: 1,
      response: 0.32,
      onUpdate: setProgress,
      onComplete: () => {
        if (closingRef.current) onCloseRef.current();
      },
    });
    springRef.current = spring;
    spring.set(1);
    return () => spring.stop();
  }, []);

  const requestClose = () => {
    if (closingRef.current) return;
    closingRef.current = true;
    if (!springRef.current) {
      onCloseRef.current();
      return;
    }
    springRef.current.set(0);
  };

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") requestClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <div
      className="modal-backdrop"
      style={{ opacity: progress }}
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) requestClose();
      }}
    >
      <div className="modal" style={{ opacity: progress, transform: `scale(${0.96 + 0.04 * progress})` }}>
        <h2>{title}</h2>
        <ModalCloseContext.Provider value={requestClose}>{children}</ModalCloseContext.Provider>
      </div>
    </div>
  );
}
