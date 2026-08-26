import { describe, expect, it } from 'vitest'
import { render } from '@testing-library/react'
import { SafeHtml, SafeHtmlNoImages } from './SafeHtml'

describe('SafeHtml', () => {
  it('sanitizes and renders HTML', () => {
    const { container } = render(
      <SafeHtml html={'<p>Hello <strong>world</strong></p><script>alert(1)</script>'} />,
    )

    expect(container.querySelector('strong')).toHaveTextContent('world')
    expect(container.querySelector('script')).not.toBeInTheDocument()
  })

  it('strips event handler attributes', () => {
    const { container } = render(<SafeHtml html={'<img src="x" onerror="alert(1)">'} />)

    const img = container.querySelector('img')
    expect(img).not.toBeNull()
    expect(img?.getAttribute('onerror')).toBeNull()
  })
})

describe('SafeHtmlNoImages', () => {
  it('strips img, picture, source, and svg tags', () => {
    const { container } = render(
      <SafeHtmlNoImages
        html={
          '<p>Kept</p>' +
          '<img src="x.png">' +
          '<picture><source srcset="x.png"><img src="y.png"></picture>' +
          '<svg><circle r="1" /></svg>'
        }
      />,
    )

    expect(container.querySelector('p')).toHaveTextContent('Kept')
    expect(container.querySelector('img')).not.toBeInTheDocument()
    expect(container.querySelector('picture')).not.toBeInTheDocument()
    expect(container.querySelector('source')).not.toBeInTheDocument()
    expect(container.querySelector('svg')).not.toBeInTheDocument()
  })

  it('strips srcset and background attributes from surviving elements', () => {
    const { container } = render(
      <SafeHtmlNoImages html={'<div background="x.png" srcset="y.png">text</div>'} />,
    )

    const div = container.querySelector('div')
    expect(div).not.toBeNull()
    expect(div?.getAttribute('background')).toBeNull()
    expect(div?.getAttribute('srcset')).toBeNull()
    expect(div).toHaveTextContent('text')
  })
})
