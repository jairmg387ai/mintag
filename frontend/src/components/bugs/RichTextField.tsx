import { forwardRef, useEffect, useImperativeHandle, useRef } from 'react'
import DOMPurify from 'dompurify'

export interface RichTextFieldHandle {
  // Returns the current, freshly-sanitized HTML content. Only called by the
  // parent at save time — never per keystroke.
  getValue(): string
}

interface RichTextFieldProps {
  initialHtml: string
  ariaLabel: string
  disabled?: boolean
  // Lightweight live signal read through the same ref used by getValue,
  // fired on every input event. Reads textContent only (no DOMPurify per
  // keystroke) and never writes back to the DOM — this does NOT make the
  // field a controlled component, it only lets the parent react to
  // emptiness (e.g. the root-cause-identified invariant) without destroying
  // the caret.
  onTextChange?: (text: string) => void
}

// Uncontrolled contentEditable rich text field. Sets innerHTML exactly once
// on mount (sanitized via DOMPurify, the same sanitizer SafeHtml.tsx uses —
// no separate DOMPurify config to keep in sync). The parent forces a fresh
// mount when the baseline identity changes (new rev after a save, or a
// different bug/field) by putting `key={`${bugId}:${rev}:${fieldName}`}` on
// the element where it renders this component — RichTextField itself never
// re-syncs its content from props after mount, since re-rendering innerHTML
// on a focused contentEditable destroys the caret position.
export const RichTextField = forwardRef<RichTextFieldHandle, RichTextFieldProps>(
  function RichTextField({ initialHtml, ariaLabel, disabled, onTextChange }, ref) {
    const divRef = useRef<HTMLDivElement>(null)

    useEffect(() => {
      if (divRef.current) {
        divRef.current.innerHTML = DOMPurify.sanitize(initialHtml)
      }
      // Intentionally runs once per mount only — the parent's `key` prop is
      // what forces a remount when the baseline should change, not this effect.
      // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [])

    useImperativeHandle(ref, () => ({
      getValue: () => DOMPurify.sanitize(divRef.current?.innerHTML ?? ''),
    }))

    return (
      <div
        ref={divRef}
        role="textbox"
        aria-label={ariaLabel}
        aria-multiline="true"
        contentEditable={!disabled}
        suppressContentEditableWarning
        onInput={onTextChange ? e => onTextChange(e.currentTarget.textContent ?? '') : undefined}
        style={{
          minHeight: 96,
          padding: '8px 12px',
          border: '1px solid var(--border-strong)',
          borderRadius: 'var(--radius-md)',
          font: 'var(--text-body)',
          color: 'var(--fg1)',
          background: disabled ? 'var(--bg-sunken)' : 'transparent',
          outline: 'none',
          overflowY: 'auto',
        }}
      />
    )
  },
)
