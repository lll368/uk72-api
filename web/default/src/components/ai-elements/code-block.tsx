/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
/* eslint-disable react-refresh/only-export-components */
'use client'

import {
  type ComponentProps,
  createContext,
  type HTMLAttributes,
  useContext,
  useState,
} from 'react'
import { CheckIcon, CopyIcon } from 'lucide-react'
import { useTheme } from 'next-themes'
import { Highlight, themes, type Language } from 'prism-react-renderer'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'

export type CodeLanguage =
  | 'bash'
  | 'python'
  | 'typescript'
  | 'javascript'
  | 'json'

type CodeBlockProps = HTMLAttributes<HTMLDivElement> & {
  code: string
  language: CodeLanguage
  showLineNumbers?: boolean
}

type CodeBlockContextType = {
  code: string
}

const CodeBlockContext = createContext<CodeBlockContextType>({
  code: '',
})

export const CodeBlock = ({
  code,
  language,
  showLineNumbers = false,
  className,
  children,
  ...props
}: CodeBlockProps) => {
  const { resolvedTheme } = useTheme()
  const prismTheme =
    resolvedTheme === 'dark' ? themes.oneDark : themes.oneLight

  return (
    <CodeBlockContext.Provider value={{ code }}>
      <div
        className={cn(
          'group bg-background text-foreground relative w-full overflow-hidden rounded-md border',
          className
        )}
        {...props}
      >
        <div className='relative'>
          <Highlight
            code={code.replace(/\n$/, '')}
            language={language as Language}
            theme={prismTheme}
          >
            {({
              className: preClassName,
              style,
              tokens,
              getLineProps,
              getTokenProps,
            }) => (
              <pre
                className={cn(
                  preClassName,
                  'm-0 overflow-hidden p-4 font-mono text-sm'
                )}
                style={style}
              >
                <code className='font-mono text-sm'>
                  {tokens.map((line, i) => {
                    const lineProps = getLineProps({ line })
                    return (
                      <div
                        key={i}
                        {...lineProps}
                        className={cn(lineProps.className)}
                      >
                        {showLineNumbers && (
                          <span className='text-muted-foreground mr-4 inline-block min-w-10 text-right select-none'>
                            {i + 1}
                          </span>
                        )}
                        {line.map((token, j) => {
                          const tokenProps = getTokenProps({ token })
                          return <span key={j} {...tokenProps} />
                        })}
                      </div>
                    )
                  })}
                </code>
              </pre>
            )}
          </Highlight>
          {children && (
            <div className='absolute top-2 right-2 flex items-center gap-2'>
              {children}
            </div>
          )}
        </div>
      </div>
    </CodeBlockContext.Provider>
  )
}

export type CodeBlockCopyButtonProps = ComponentProps<typeof Button> & {
  onCopy?: () => void
  onError?: (error: Error) => void
  timeout?: number
}

export const CodeBlockCopyButton = ({
  onCopy,
  onError,
  timeout = 2000,
  children,
  className,
  ...props
}: CodeBlockCopyButtonProps) => {
  const [isCopied, setIsCopied] = useState(false)
  const { code } = useContext(CodeBlockContext)

  const copyToClipboard = async () => {
    if (typeof window === 'undefined' || !navigator?.clipboard?.writeText) {
      onError?.(new Error('Clipboard API not available'))
      return
    }

    try {
      await navigator.clipboard.writeText(code)
      setIsCopied(true)
      onCopy?.()
      setTimeout(() => setIsCopied(false), timeout)
    } catch (error) {
      onError?.(error as Error)
    }
  }

  const Icon = isCopied ? CheckIcon : CopyIcon

  return (
    <Button
      className={cn('shrink-0', className)}
      onClick={copyToClipboard}
      size='icon'
      variant='ghost'
      {...props}
    >
      {children ?? <Icon size={14} />}
    </Button>
  )
}
