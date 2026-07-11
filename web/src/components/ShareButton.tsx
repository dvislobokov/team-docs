import { useState } from "react";
import { Check, Link2 } from "lucide-react";

// «Поделиться» = копировать ссылку на текущую страницу в буфер обмена.
// Аутентификации/прав нет, поэтому это просто удобный шаринг ссылки.
export function ShareButton() {
  const [copied, setCopied] = useState(false);

  const share = async () => {
    try {
      await navigator.clipboard.writeText(window.location.href);
    } catch {
      // старый фолбэк, если clipboard недоступен
      const ta = document.createElement("textarea");
      ta.value = window.location.href;
      document.body.appendChild(ta);
      ta.select();
      document.execCommand("copy");
      ta.remove();
    }
    setCopied(true);
    setTimeout(() => setCopied(false), 1600);
  };

  return (
    <button
      type="button"
      onClick={share}
      className="flex items-center gap-1.5 rounded-md px-3 py-1.5 text-[13px] font-500 text-body transition hover:bg-line/60"
      title="Скопировать ссылку"
    >
      {copied ? <Check className="h-3.5 w-3.5 text-accent" /> : <Link2 className="h-3.5 w-3.5" />}
      {copied ? "Скопировано" : "Поделиться"}
    </button>
  );
}
