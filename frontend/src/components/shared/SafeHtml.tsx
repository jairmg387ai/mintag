import DOMPurify from 'dompurify'

/**
 * Renders sanitized HTML via DOMPurify. Shared by any view that displays
 * rich/HTML content coming from a meeting or an external source, so
 * sanitization rules live in exactly one place.
 */
export function SafeHtml({ html }: { html: string }) {
  return (
    <div
      style={{ font: 'var(--text-body)', lineHeight: 1.7, color: 'var(--fg2)' }}
      dangerouslySetInnerHTML={{ __html: DOMPurify.sanitize(html) }}
    />
  )
}

/**
 * Same sanitization as SafeHtml, but additionally strips image-carrying
 * tags/attributes (img/picture/source/svg, srcset/background) — for content
 * sources where remote image loading is not desired (e.g. read-only evidence
 * comments rendered outside the user's own trusted meeting notes).
 */
export function SafeHtmlNoImages({ html }: { html: string }) {
  const sanitized = DOMPurify.sanitize(html, {
    FORBID_TAGS: ['img', 'picture', 'source', 'svg'],
    FORBID_ATTR: ['srcset', 'background'],
  })
  return (
    <div
      style={{ font: 'var(--text-body)', lineHeight: 1.7, color: 'var(--fg2)' }}
      dangerouslySetInnerHTML={{ __html: sanitized }}
    />
  )
}
