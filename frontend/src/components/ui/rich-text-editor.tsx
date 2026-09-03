'use client';

import React, { useRef, useEffect } from 'react';
import { Bold, Italic, List } from 'lucide-react';
import { cn } from '@/lib/utils';

interface RichTextEditorProps {
    content: string;
    onChange: (html: string, text: string) => void;
    placeholder?: string;
    minHeight?: string;
}

export const RichTextEditor: React.FC<RichTextEditorProps> = ({ content, onChange, placeholder, minHeight = '100px' }) => {
    const editorRef = useRef<HTMLDivElement>(null);

    useEffect(() => {
        if (editorRef.current && editorRef.current.innerHTML !== content) {
            editorRef.current.innerHTML = content || '';
        }
        // Only sync when content is set from outside (e.g. loading an existing task);
        // typing already updates the DOM directly, so re-running this on every
        // keystroke would fight the browser's own caret position.
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);

    const emitChange = () => {
        const el = editorRef.current;
        if (!el) return;
        onChange(el.innerHTML, el.textContent || '');
    };

    const exec = (command: string) => {
        document.execCommand(command);
        editorRef.current?.focus();
        emitChange();
    };

    return (
        <div className="border border-gray-300 rounded-lg overflow-hidden">
            <div className="flex items-center gap-1 border-b border-gray-200 bg-gray-50 px-2 py-1">
                <button
                    type="button"
                    onClick={() => exec('bold')}
                    className="p-1.5 rounded hover:bg-gray-200 text-gray-600"
                >
                    <Bold size={14} />
                </button>
                <button
                    type="button"
                    onClick={() => exec('italic')}
                    className="p-1.5 rounded hover:bg-gray-200 text-gray-600"
                >
                    <Italic size={14} />
                </button>
                <button
                    type="button"
                    onClick={() => exec('insertUnorderedList')}
                    className="p-1.5 rounded hover:bg-gray-200 text-gray-600"
                >
                    <List size={14} />
                </button>
            </div>
            <div
                ref={editorRef}
                contentEditable
                onInput={emitChange}
                data-placeholder={placeholder}
                className={cn(
                    'p-3 text-sm text-gray-900 focus:outline-none prose prose-sm max-w-none',
                    'empty:before:content-[attr(data-placeholder)] empty:before:text-gray-400'
                )}
                style={{ minHeight }}
            />
        </div>
    );
};
