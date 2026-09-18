// frontend/src/components/emails/ComposeEditor.tsx
'use client'

import { forwardRef, useEffect, useImperativeHandle, useRef } from 'react'
import { useEditor, EditorContent, type Editor } from '@tiptap/react'
import StarterKit from '@tiptap/starter-kit'
import Underline from '@tiptap/extension-underline'
import Link from '@tiptap/extension-link'
import { TextStyle } from '@tiptap/extension-text-style'
import Color from '@tiptap/extension-color'
import TextAlign from '@tiptap/extension-text-align'
import Placeholder from '@tiptap/extension-placeholder'
import {
    Bold, Italic, Underline as UnderlineIcon, List, ListOrdered, Quote,
    AlignLeft, AlignCenter, AlignRight, Link as LinkIcon, Link2Off, Eraser,
} from 'lucide-react'

const TEXT_COLORS = ['#0f172a', '#dc2626', '#16a34a', '#2563eb', '#d97706', '#7c3aed']

export interface ComposeEditorHandle {
    getHTML: () => string
    getText: () => string
    isEmpty: () => boolean
}

function ToolbarButton({
    onClick, active, disabled, title, children,
}: {
    onClick: () => void
    active?: boolean
    disabled?: boolean
    title: string
    children: React.ReactNode
}) {
    return (
        <button
            type="button"
            title={title}
            disabled={disabled}
            onClick={onClick}
            className={`p-1.5 rounded-md transition-colors disabled:opacity-40 ${
                active ? 'bg-primary/15 text-primary' : 'text-muted-foreground hover:bg-muted hover:text-foreground'
            }`}
        >
            {children}
        </button>
    )
}

function Toolbar({ editor }: { editor: Editor }) {
    const colorInputRef = useRef<HTMLInputElement>(null)

    const setLink = () => {
        const previousUrl = editor.getAttributes('link').href as string | undefined
        const url = window.prompt('Link URL-je:', previousUrl || 'https://')
        if (url === null) return
        if (url === '') {
            editor.chain().focus().unsetLink().run()
            return
        }
        editor.chain().focus().extendMarkRange('link').setLink({ href: url }).run()
    }

    return (
        <div className="flex flex-wrap items-center gap-0.5 border border-input rounded-t-lg bg-muted/40 px-2 py-1.5">
            <ToolbarButton title="Félkövér" active={editor.isActive('bold')} onClick={() => editor.chain().focus().toggleBold().run()}>
                <Bold size={15} />
            </ToolbarButton>
            <ToolbarButton title="Dőlt" active={editor.isActive('italic')} onClick={() => editor.chain().focus().toggleItalic().run()}>
                <Italic size={15} />
            </ToolbarButton>
            <ToolbarButton title="Aláhúzott" active={editor.isActive('underline')} onClick={() => editor.chain().focus().toggleUnderline().run()}>
                <UnderlineIcon size={15} />
            </ToolbarButton>
            <div className="w-px h-5 bg-border mx-1" />
            <ToolbarButton title="Felsorolás" active={editor.isActive('bulletList')} onClick={() => editor.chain().focus().toggleBulletList().run()}>
                <List size={15} />
            </ToolbarButton>
            <ToolbarButton title="Számozott lista" active={editor.isActive('orderedList')} onClick={() => editor.chain().focus().toggleOrderedList().run()}>
                <ListOrdered size={15} />
            </ToolbarButton>
            <ToolbarButton title="Idézetblokk" active={editor.isActive('blockquote')} onClick={() => editor.chain().focus().toggleBlockquote().run()}>
                <Quote size={15} />
            </ToolbarButton>
            <div className="w-px h-5 bg-border mx-1" />
            <ToolbarButton title="Balra igazítás" active={editor.isActive({ textAlign: 'left' })} onClick={() => editor.chain().focus().setTextAlign('left').run()}>
                <AlignLeft size={15} />
            </ToolbarButton>
            <ToolbarButton title="Középre igazítás" active={editor.isActive({ textAlign: 'center' })} onClick={() => editor.chain().focus().setTextAlign('center').run()}>
                <AlignCenter size={15} />
            </ToolbarButton>
            <ToolbarButton title="Jobbra igazítás" active={editor.isActive({ textAlign: 'right' })} onClick={() => editor.chain().focus().setTextAlign('right').run()}>
                <AlignRight size={15} />
            </ToolbarButton>
            <div className="w-px h-5 bg-border mx-1" />
            <ToolbarButton title="Link beszúrása" active={editor.isActive('link')} onClick={setLink}>
                <LinkIcon size={15} />
            </ToolbarButton>
            <ToolbarButton title="Link eltávolítása" disabled={!editor.isActive('link')} onClick={() => editor.chain().focus().unsetLink().run()}>
                <Link2Off size={15} />
            </ToolbarButton>
            <div className="w-px h-5 bg-border mx-1" />
            <div className="flex items-center gap-0.5">
                {TEXT_COLORS.map(color => (
                    <button
                        key={color}
                        type="button"
                        title={color}
                        onClick={() => editor.chain().focus().setColor(color).run()}
                        className="w-4 h-4 rounded-full border border-border/60"
                        style={{ backgroundColor: color }}
                    />
                ))}
                <button
                    type="button"
                    title="Egyéni szín"
                    onClick={() => colorInputRef.current?.click()}
                    className="w-4 h-4 rounded-full border border-border/60 bg-[conic-gradient(red,yellow,lime,cyan,blue,magenta,red)]"
                />
                <input
                    ref={colorInputRef}
                    type="color"
                    className="hidden"
                    onChange={e => editor.chain().focus().setColor(e.target.value).run()}
                />
            </div>
            <div className="w-px h-5 bg-border mx-1" />
            <ToolbarButton title="Formázás törlése" onClick={() => editor.chain().focus().unsetAllMarks().clearNodes().run()}>
                <Eraser size={15} />
            </ToolbarButton>
        </div>
    )
}

const ComposeEditor = forwardRef<ComposeEditorHandle, { placeholder?: string }>(function ComposeEditor(
    { placeholder },
    ref,
) {
    const editor = useEditor({
        immediatelyRender: false,
        extensions: [
            StarterKit,
            Underline,
            TextStyle,
            Color,
            TextAlign.configure({ types: ['heading', 'paragraph'] }),
            Link.configure({ openOnClick: false, autolink: true }),
            Placeholder.configure({ placeholder: placeholder || 'Írd ide az üzenetet...' }),
        ],
        content: '',
        editorProps: {
            attributes: {
                class: 'prose prose-sm max-w-none min-h-[140px] px-3 py-2 focus:outline-none',
            },
        },
    })

    useImperativeHandle(ref, () => ({
        getHTML: () => editor?.getHTML() ?? '',
        getText: () => editor?.getText() ?? '',
        isEmpty: () => editor?.isEmpty ?? true,
    }), [editor])

    useEffect(() => () => editor?.destroy(), [editor])

    if (!editor) return null

    return (
        <div>
            <Toolbar editor={editor} />
            <div className="border border-t-0 border-input rounded-b-lg">
                <EditorContent editor={editor} />
            </div>
        </div>
    )
})

export default ComposeEditor
