import { useEffect, useRef, useState, type ReactNode } from "react";
import { EmojiPicker } from "./EmojiPicker";

interface EmojiButtonProps {
  /** Текущий emoji ("" — иконки нет). */
  value: string;
  onChange: (emoji: string) => void;
  /** Что показать в триггере, когда emoji не выбран. */
  fallback: ReactNode;
  triggerClassName?: string;
  /** Разрешить снять иконку (кнопка «Убрать» в пикере). */
  allowRemove?: boolean;
  /** Только просмотр: показываем иконку без пикара (нет прав на правку). */
  disabled?: boolean;
}

// Триггер-иконка с всплывающим emoji-пикером. Закрывается по клику вне и Esc.
export function EmojiButton({
  value,
  onChange,
  fallback,
  triggerClassName,
  allowRemove = true,
  disabled = false,
}: EmojiButtonProps) {
  const [open, setOpen] = useState(false);
  const wrapRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (wrapRef.current && !wrapRef.current.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("mousedown", onDown);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDown);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  if (disabled) {
    return (
      <span className={triggerClassName} aria-hidden>
        {value || fallback}
      </span>
    );
  }

  return (
    <div ref={wrapRef} className="relative inline-block">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className={triggerClassName}
        title="Сменить иконку"
      >
        {value || fallback}
      </button>

      {open && (
        <div className="animate-fade-in absolute left-0 top-full z-50 mt-2">
          <EmojiPicker
            onSelect={(emoji) => {
              onChange(emoji);
              setOpen(false);
            }}
            onRemove={
              allowRemove && value
                ? () => {
                    onChange("");
                    setOpen(false);
                  }
                : undefined
            }
          />
        </div>
      )}
    </div>
  );
}
