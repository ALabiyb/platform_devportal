// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
import { useEffect, useRef, useState, useCallback } from "react";

const IDLE_MS  = 30 * 60 * 1000; // 30 minutes of inactivity triggers logout
const WARN_MS  =  2 * 60 * 1000; // show warning 2 minutes before logout

const ACTIVITY_EVENTS = ["mousemove", "mousedown", "keydown", "touchstart", "scroll", "click"] as const;

export interface IdleTimeoutState {
  warningVisible: boolean;
  secondsLeft: number;
  stayLoggedIn: () => void;
}

export function useIdleTimeout(onLogout: () => void): IdleTimeoutState {
  const [warningVisible, setWarningVisible] = useState(false);
  const [secondsLeft, setSecondsLeft] = useState(WARN_MS / 1000);

  const warnTimer    = useRef<ReturnType<typeof setTimeout>  | null>(null);
  const logoutTimer  = useRef<ReturnType<typeof setTimeout>  | null>(null);
  const countdown    = useRef<ReturnType<typeof setInterval> | null>(null);
  const onLogoutRef  = useRef(onLogout);

  useEffect(() => { onLogoutRef.current = onLogout; }, [onLogout]);

  const clearAll = useCallback(() => {
    if (warnTimer.current)   clearTimeout(warnTimer.current);
    if (logoutTimer.current) clearTimeout(logoutTimer.current);
    if (countdown.current)   clearInterval(countdown.current);
  }, []);

  const reset = useCallback(() => {
    clearAll();
    setWarningVisible(false);

    warnTimer.current = setTimeout(() => {
      setWarningVisible(true);
      setSecondsLeft(WARN_MS / 1000);
      countdown.current = setInterval(() => {
        setSecondsLeft((s) => {
          if (s <= 1) { clearInterval(countdown.current!); return 0; }
          return s - 1;
        });
      }, 1000);
    }, IDLE_MS - WARN_MS);

    logoutTimer.current = setTimeout(() => {
      clearAll();
      onLogoutRef.current();
    }, IDLE_MS);
  }, [clearAll]);

  const resetRef = useRef(reset);
  useEffect(() => { resetRef.current = reset; }, [reset]);

  useEffect(() => {
    const handler = () => resetRef.current();
    ACTIVITY_EVENTS.forEach((e) => window.addEventListener(e, handler, { passive: true }));
    resetRef.current();
    return () => {
      clearAll();
      ACTIVITY_EVENTS.forEach((e) => window.removeEventListener(e, handler));
    };
  }, [clearAll]);

  return { warningVisible, secondsLeft, stayLoggedIn: reset };
}
