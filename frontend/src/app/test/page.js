// pages/test.js (or app/test/page.js for App Router)
'use client'; // Only needed if using App Router

import React, { useState, useRef } from 'react';
import { Plus, Edit2, MessageSquare, X, Save, Trash2, GripVertical, Clock, Timer, Calendar, User, AlertCircle, CheckCircle, BarChart3, Bold, Italic, Link, List, Quote, Underline, Code, Image, AlignLeft, AlignCenter, AlignRight, Type } from 'lucide-react';

const KanbanBoard = () => {
    const [columns, setColumns] = useState([
        { id: 'todo', title: 'Tennivalók', tasks: [], color: 'bg-blue-500' },
        { id: 'in-progress', title: 'Folyamatban', tasks: [], color: 'bg-yellow-500' },
        { id: 'review', title: 'Áttekintés', tasks: [], color: 'bg-purple-500' },
        { id: 'done', title: 'Kész', tasks: [], color: 'bg-green-500' }
    ]);

    const [draggedTask, setDraggedTask] = useState(null);
    const [draggedFrom, setDraggedFrom] = useState(null);
    const [editingTask, setEditingTask] = useState(null);
    const [showComments, setShowComments] = useState(null);
    const [showTimeLog, setShowTimeLog] = useState(null);
    const [taskIdCounter, setTaskIdCounter] = useState(1);

    // Új feladat hozzáadása
    const addTask = (columnId) => {
        const newTask = {
            id: `task-${taskIdCounter}`,
            title: 'Új feladat',
            description: 'Feladat leírása...',
            htmlDescription: 'Feladat leírása...',
            priority: 'medium',
            assignee: '',
            estimatedHours: 0,
            loggedHours: 0,
            timeEntries: [],
            comments: [],
            createdAt: new Date().toISOString(),
            tags: []
        };

        setColumns(prev => prev.map(col =>
            col.id === columnId
                ? { ...col, tasks: [...col.tasks, newTask] }
                : col
        ));

        setTaskIdCounter(prev => prev + 1);
        setEditingTask(newTask.id);
    };

    // Feladat frissítése
    const updateTask = (taskId, updates) => {
        setColumns(prev => prev.map(col => ({
            ...col,
            tasks: col.tasks.map(task =>
                task.id === taskId ? { ...task, ...updates } : task
            )
        })));
    };

    // Feladat törlése
    const deleteTask = (taskId) => {
        setColumns(prev => prev.map(col => ({
            ...col,
            tasks: col.tasks.filter(task => task.id !== taskId)
        })));
    };

    // Komment hozzáadása
    const addComment = (taskId, commentData) => {
        const newComment = {
            id: Date.now(),
            ...commentData,
            author: 'Felhasználó',
            timestamp: new Date().toISOString(),
            edited: false,
            htmlContent: commentData.htmlContent || commentData.text
        };

        updateTask(taskId, {
            comments: [...getTask(taskId).comments, newComment]
        });
    };

    // Idő bejegyzés hozzáadása
    const addTimeEntry = (taskId, timeData) => {
        const newTimeEntry = {
            id: Date.now(),
            ...timeData,
            timestamp: new Date().toISOString()
        };

        const task = getTask(taskId);
        const newLoggedHours = task.loggedHours + timeData.hours;

        updateTask(taskId, {
            timeEntries: [...task.timeEntries, newTimeEntry],
            loggedHours: newLoggedHours
        });
    };

    // Feladat megkeresése
    const getTask = (taskId) => {
        for (const col of columns) {
            const task = col.tasks.find(t => t.id === taskId);
            if (task) return task;
        }
        return null;
    };

    // Drag and Drop kezelés
    const [dragOverColumn, setDragOverColumn] = useState(null);

    const handleDragStart = (e, task, fromColumnId) => {
        setDraggedTask(task);
        setDraggedFrom(fromColumnId);

        e.dataTransfer.setData('application/json', JSON.stringify({
            taskId: task.id,
            fromColumn: fromColumnId
        }));
        e.dataTransfer.effectAllowed = 'move';
    };

    const handleDragOver = (e) => {
        e.preventDefault();
        e.dataTransfer.dropEffect = 'move';
    };

    const handleDragEnter = (e, columnId) => {
        e.preventDefault();
        setDragOverColumn(columnId);
    };

    const handleDragLeave = (e) => {
        if (!e.currentTarget.contains(e.relatedTarget)) {
            setDragOverColumn(null);
        }
    };

    const handleDrop = (e, toColumnId) => {
        e.preventDefault();
        setDragOverColumn(null);

        let taskId, fromColumnId;

        try {
            const data = e.dataTransfer.getData('application/json');
            if (data) {
                const parsed = JSON.parse(data);
                taskId = parsed.taskId;
                fromColumnId = parsed.fromColumn;
            }
        } catch (err) {
            taskId = draggedTask?.id;
            fromColumnId = draggedFrom;
        }

        if (taskId && fromColumnId && fromColumnId !== toColumnId) {
            let taskToMove = null;
            for (const col of columns) {
                const found = col.tasks.find(t => t.id === taskId);
                if (found) {
                    taskToMove = found;
                    break;
                }
            }

            if (taskToMove) {
                setColumns(prev => prev.map(col => {
                    if (col.id === fromColumnId) {
                        return { ...col, tasks: col.tasks.filter(t => t.id !== taskId) };
                    }
                    if (col.id === toColumnId) {
                        return { ...col, tasks: [...col.tasks, taskToMove] };
                    }
                    return col;
                }));
            }
        }

        setDraggedTask(null);
        setDraggedFrom(null);
    };

    // Prioritás színek
    const getPriorityColor = (priority) => {
        switch (priority) {
            case 'urgent': return 'bg-red-50 border-red-300 shadow-red-100';
            case 'high': return 'bg-orange-50 border-orange-300 shadow-orange-100';
            case 'medium': return 'bg-yellow-50 border-yellow-300 shadow-yellow-100';
            case 'low': return 'bg-green-50 border-green-300 shadow-green-100';
            default: return 'bg-gray-50 border-gray-300 shadow-gray-100';
        }
    };

    const getPriorityIcon = (priority) => {
        switch (priority) {
            case 'urgent': return <AlertCircle className="w-4 h-4 text-red-500" />;
            case 'high': return <AlertCircle className="w-4 h-4 text-orange-500" />;
            case 'medium': return <Clock className="w-4 h-4 text-yellow-500" />;
            case 'low': return <CheckCircle className="w-4 h-4 text-green-500" />;
            default: return <Clock className="w-4 h-4 text-gray-500" />;
        }
    };

    return (
        <div className="min-h-screen bg-gradient-to-br from-gray-50 to-gray-100 p-6">
            <div className="max-w-7xl mx-auto">
                <div className="mb-8">
                    <h1 className="text-4xl font-bold text-gray-800 mb-2">Kanban Board</h1>
                    <p className="text-gray-600">Projektmenedzsment drag & drop funkcióval</p>
                </div>

                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
                    {columns.map((column) => (
                        <div key={column.id} className="bg-white rounded-xl shadow-lg border border-gray-200 overflow-hidden">
                            <div className={`p-4 ${column.color} bg-opacity-10 border-b border-gray-200`}>
                                <div className="flex items-center justify-between">
                                    <div className="flex items-center gap-2">
                                        <div className={`w-3 h-3 rounded-full ${column.color}`}></div>
                                        <h2 className="font-semibold text-gray-800">{column.title}</h2>
                                    </div>
                                    <span className="bg-white bg-opacity-80 text-gray-700 px-3 py-1 rounded-full text-sm font-medium">
                    {column.tasks.length}
                  </span>
                                </div>
                                <button
                                    onClick={() => addTask(column.id)}
                                    className="mt-3 w-full flex items-center justify-center gap-2 p-3 text-gray-600 hover:text-blue-600 hover:bg-blue-50 rounded-lg transition-all duration-200 border-2 border-dashed border-gray-300 hover:border-blue-300"
                                >
                                    <Plus size={18} />
                                    Új feladat
                                </button>
                            </div>

                            <div
                                className={`p-4 min-h-[500px] space-y-4 transition-colors ${
                                    dragOverColumn === column.id ? 'bg-blue-50 border-blue-200' : ''
                                }`}
                                onDragOver={handleDragOver}
                                onDragEnter={(e) => handleDragEnter(e, column.id)}
                                onDragLeave={handleDragLeave}
                                onDrop={(e) => handleDrop(e, column.id)}
                            >
                                {column.tasks.map((task) => (
                                    <TaskCard
                                        key={task.id}
                                        task={task}
                                        columnId={column.id}
                                        onDragStart={(e) => handleDragStart(e, task, column.id)}
                                        onDragEnd={() => {}}
                                        onEdit={() => setEditingTask(task.id)}
                                        onDelete={() => deleteTask(task.id)}
                                        onShowComments={() => setShowComments(task.id)}
                                        onShowTimeLog={() => setShowTimeLog(task.id)}
                                        isEditing={editingTask === task.id}
                                        onSave={(updates) => {
                                            updateTask(task.id, updates);
                                            setEditingTask(null);
                                        }}
                                        onCancel={() => setEditingTask(null)}
                                        getPriorityColor={getPriorityColor}
                                        getPriorityIcon={getPriorityIcon}
                                    />
                                ))}
                            </div>
                        </div>
                    ))}
                </div>

                {/* Komment Modal */}
                {showComments && (
                    <CommentModal
                        task={getTask(showComments)}
                        onClose={() => setShowComments(null)}
                        onAddComment={(commentData) => addComment(showComments, commentData)}
                    />
                )}

                {/* Idő nyilvántartás Modal */}
                {showTimeLog && (
                    <TimeLogModal
                        task={getTask(showTimeLog)}
                        onClose={() => setShowTimeLog(null)}
                        onAddTimeEntry={(timeData) => addTimeEntry(showTimeLog, timeData)}
                        onUpdateEstimate={(estimate) => updateTask(showTimeLog, { estimatedHours: estimate })}
                    />
                )}
            </div>
        </div>
    );
};

// Rich Text Editor komponens
const RichTextEditor = ({ content, onChange, placeholder = "Szöveg írása...", minHeight = "120px" }) => {
    const editorRef = useRef(null);
    const fileInputRef = useRef(null);
    const [currentFormat, setCurrentFormat] = useState({
        bold: false,
        italic: false,
        underline: false
    });

    const [localContent, setLocalContent] = useState(content);

    React.useEffect(() => {
        setLocalContent(content);
    }, [content]);

    const applyFormat = (command, value = null) => {
        if (editorRef.current) {
            editorRef.current.focus();
            document.execCommand(command, false, value);

            setTimeout(() => {
                if (editorRef.current) {
                    const html = editorRef.current.innerHTML;
                    const text = editorRef.current.textContent || '';
                    setLocalContent(html);
                    onChange(html, text);
                    updateFormatState();
                }
            }, 10);
        }
    };

    const updateFormatState = () => {
        if (editorRef.current && document.activeElement === editorRef.current) {
            setCurrentFormat({
                bold: document.queryCommandState('bold'),
                italic: document.queryCommandState('italic'),
                underline: document.queryCommandState('underline')
            });
        }
    };

    const handleInput = () => {
        if (editorRef.current) {
            const html = editorRef.current.innerHTML;
            const text = editorRef.current.textContent || '';
            setLocalContent(html);

            clearTimeout(window.editorUpdateTimeout);
            window.editorUpdateTimeout = setTimeout(() => {
                onChange(html, text);
            }, 300);
        }
    };

    const handleImageUpload = (e) => {
        const file = e.target.files[0];
        if (file && file.type.startsWith('image/')) {
            const reader = new FileReader();
            reader.onload = (e) => {
                const img = `<img src="${e.target.result}" alt="Feltöltött kép" style="max-width: 100%; height: auto; border-radius: 8px; margin: 8px 0;" />`;
                applyFormat('insertHTML', img);
            };
            reader.readAsDataURL(file);
        }
    };

    const insertList = (ordered = false) => {
        applyFormat(ordered ? 'insertOrderedList' : 'insertUnorderedList');
    };

    const insertLink = () => {
        const url = prompt('Add meg a link URL-t:');
        if (url) {
            applyFormat('createLink', url);
        }
    };

    const formatButtons = [
        { icon: Bold, command: 'bold', active: currentFormat.bold, title: 'Félkövér (Ctrl+B)' },
        { icon: Italic, command: 'italic', active: currentFormat.italic, title: 'Dőlt (Ctrl+I)' },
        { icon: Underline, command: 'underline', active: currentFormat.underline, title: 'Aláhúzott (Ctrl+U)' },
    ];

    return (
        <div className="border border-gray-300 rounded-lg overflow-hidden focus-within:ring-2 focus-within:ring-blue-500 focus-within:border-transparent">
            <div className="bg-gray-50 border-b border-gray-200 p-2 flex flex-wrap gap-1">
                <div className="flex gap-1 border-r border-gray-300 pr-2 mr-2">
                    {formatButtons.map(({ icon: Icon, command, active, title }) => (
                        <button
                            key={command}
                            type="button"
                            onMouseDown={(e) => e.preventDefault()}
                            onClick={() => applyFormat(command)}
                            className={`p-2 rounded transition-colors ${
                                active ? 'bg-blue-500 text-white' : 'text-gray-600 hover:bg-gray-200'
                            }`}
                            title={title}
                        >
                            <Icon size={16} />
                        </button>
                    ))}
                </div>

                <div className="flex gap-1 border-r border-gray-300 pr-2 mr-2">
                    <button
                        type="button"
                        onMouseDown={(e) => e.preventDefault()}
                        onClick={() => insertList(false)}
                        className="p-2 rounded text-gray-600 hover:bg-gray-200 transition-colors"
                        title="Felsorolás"
                    >
                        <List size={16} />
                    </button>
                    <button
                        type="button"
                        onMouseDown={(e) => e.preventDefault()}
                        onClick={() => insertList(true)}
                        className="p-2 rounded text-gray-600 hover:bg-gray-200 transition-colors"
                        title="Számozott lista"
                    >
                        <Type size={16} />
                    </button>
                </div>

                <div className="flex gap-1">
                    <button
                        type="button"
                        onMouseDown={(e) => e.preventDefault()}
                        onClick={insertLink}
                        className="p-2 rounded text-gray-600 hover:bg-gray-200 transition-colors"
                        title="Link beszúrása"
                    >
                        <Link size={16} />
                    </button>
                    <button
                        type="button"
                        onMouseDown={(e) => e.preventDefault()}
                        onClick={() => applyFormat('formatBlock', 'blockquote')}
                        className="p-2 rounded text-gray-600 hover:bg-gray-200 transition-colors"
                        title="Idézet"
                    >
                        <Quote size={16} />
                    </button>
                    <button
                        type="button"
                        onMouseDown={(e) => e.preventDefault()}
                        onClick={() => fileInputRef.current?.click()}
                        className="p-2 rounded text-gray-600 hover:bg-gray-200 transition-colors"
                        title="Kép feltöltése"
                    >
                        <Image size={16} />
                    </button>
                </div>
            </div>

            <div
                ref={editorRef}
                contentEditable
                suppressContentEditableWarning={true}
                onInput={handleInput}
                onMouseUp={updateFormatState}
                onKeyUp={updateFormatState}
                onFocus={updateFormatState}
                className="p-3 focus:outline-none prose prose-sm max-w-none"
                style={{
                    minHeight: minHeight,
                }}
                dangerouslySetInnerHTML={{ __html: localContent }}
                data-placeholder={placeholder}
            />

            <input
                ref={fileInputRef}
                type="file"
                accept="image/*"
                onChange={handleImageUpload}
                className="hidden"
            />
        </div>
    );
};

// Feladat kártya komponens
const TaskCard = ({
                      task,
                      columnId,
                      onDragStart,
                      onDragEnd,
                      onEdit,
                      onDelete,
                      onShowComments,
                      onShowTimeLog,
                      isEditing,
                      onSave,
                      onCancel,
                      getPriorityColor,
                      getPriorityIcon
                  }) => {
    const [editData, setEditData] = useState({
        title: task.title,
        description: task.description,
        htmlDescription: task.htmlDescription || task.description,
        priority: task.priority,
        assignee: task.assignee,
        estimatedHours: task.estimatedHours,
        tags: task.tags || []
    });

    const handleSave = () => {
        onSave(editData);
    };

    const progressPercentage = task.estimatedHours > 0 ? Math.min((task.loggedHours / task.estimatedHours) * 100, 100) : 0;

    if (isEditing) {
        return (
            <div className={`p-6 rounded-xl border-2 ${getPriorityColor(editData.priority)} shadow-lg space-y-4`}>
                <input
                    type="text"
                    value={editData.title}
                    onChange={(e) => setEditData(prev => ({ ...prev, title: e.target.value }))}
                    className="w-full p-3 border border-gray-300 rounded-lg text-sm font-medium focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                    placeholder="Feladat címe"
                />

                <RichTextEditor
                    content={editData.htmlDescription}
                    onChange={(html, text) => setEditData(prev => ({
                        ...prev,
                        htmlDescription: html,
                        description: text
                    }))}
                    placeholder="Feladat leírása"
                    minHeight="100px"
                />

                <div className="grid grid-cols-2 gap-3">
                    <select
                        value={editData.priority}
                        onChange={(e) => setEditData(prev => ({ ...prev, priority: e.target.value }))}
                        className="p-3 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                    >
                        <option value="low">Alacsony</option>
                        <option value="medium">Közepes</option>
                        <option value="high">Magas</option>
                        <option value="urgent">Sürgős</option>
                    </select>

                    <input
                        type="number"
                        value={editData.estimatedHours}
                        onChange={(e) => setEditData(prev => ({ ...prev, estimatedHours: parseFloat(e.target.value) || 0 }))}
                        className="p-3 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                        placeholder="Becsült óra"
                        min="0"
                        step="0.5"
                    />
                </div>

                <input
                    type="text"
                    value={editData.assignee}
                    onChange={(e) => setEditData(prev => ({ ...prev, assignee: e.target.value }))}
                    className="w-full p-3 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                    placeholder="Felelős személy"
                />

                <div className="flex gap-2">
                    <button
                        onClick={handleSave}
                        className="flex-1 flex items-center justify-center gap-2 px-4 py-3 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors text-sm font-medium"
                    >
                        <Save size={16} />
                        Mentés
                    </button>
                    <button
                        onClick={onCancel}
                        className="flex-1 flex items-center justify-center gap-2 px-4 py-3 bg-gray-500 text-white rounded-lg hover:bg-gray-600 transition-colors text-sm font-medium"
                    >
                        <X size={16} />
                        Mégse
                    </button>
                </div>
            </div>
        );
    }

    return (
        <div
            draggable={!isEditing}
            onDragStart={(e) => !isEditing && onDragStart(e, task, columnId)}
            onDragEnd={onDragEnd}
            className={`p-4 rounded-xl border-2 ${getPriorityColor(task.priority)} cursor-move hover:shadow-lg transition-all duration-200 group backdrop-blur-sm`}
        >
            <div className="flex items-start justify-between mb-3">
                <div className="flex items-center gap-2">
                    {getPriorityIcon(task.priority)}
                    <h3 className="font-semibold text-gray-800 text-sm">{task.title}</h3>
                </div>
                <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                    <GripVertical size={16} className="text-gray-400" />
                </div>
            </div>

            <div className="text-gray-600 text-sm mb-4 line-clamp-3 prose prose-sm max-w-none">
                <div dangerouslySetInnerHTML={{ __html: task.htmlDescription || task.description }} />
            </div>

            {/* Idő tracking */}
            {task.estimatedHours > 0 && (
                <div className="mb-4">
                    <div className="flex items-center justify-between text-xs text-gray-600 mb-1">
                        <span>Időkeret</span>
                        <span>{task.loggedHours}h / {task.estimatedHours}h</span>
                    </div>
                    <div className="w-full bg-gray-200 rounded-full h-2">
                        <div
                            className={`h-2 rounded-full transition-all duration-300 ${
                                progressPercentage > 100 ? 'bg-red-500' :
                                    progressPercentage > 80 ? 'bg-yellow-500' : 'bg-green-500'
                            }`}
                            style={{ width: `${Math.min(progressPercentage, 100)}%` }}
                        ></div>
                    </div>
                </div>
            )}

            {/* Assignee és Priority */}
            <div className="flex items-center justify-between mb-4">
                {task.assignee && (
                    <div className="flex items-center gap-2">
                        <User size={14} className="text-gray-500" />
                        <span className="text-xs text-gray-600">{task.assignee}</span>
                    </div>
                )}
                <div className="flex items-center gap-1">
                    <Clock size={12} className="text-gray-400" />
                    <span className="text-xs text-gray-500">{task.loggedHours}h</span>
                </div>
            </div>

            {/* Műveletek */}
            <div className="flex items-center justify-between pt-3 border-t border-gray-200">
                <div className="flex items-center gap-2">
                    <button
                        onClick={onEdit}
                        className="p-2 text-gray-500 hover:text-blue-600 hover:bg-blue-50 rounded-lg transition-colors"
                        title="Szerkesztés"
                    >
                        <Edit2 size={16} />
                    </button>
                    <button
                        onClick={onShowComments}
                        className="p-2 text-gray-500 hover:text-green-600 hover:bg-green-50 rounded-lg transition-colors relative"
                        title="Kommentek"
                    >
                        <MessageSquare size={16} />
                        {task.comments.length > 0 && (
                            <span className="absolute -top-1 -right-1 bg-red-500 text-white text-xs rounded-full w-5 h-5 flex items-center justify-center">
                {task.comments.length}
              </span>
                        )}
                    </button>
                    <button
                        onClick={onShowTimeLog}
                        className="p-2 text-gray-500 hover:text-purple-600 hover:bg-purple-50 rounded-lg transition-colors"
                        title="Idő nyilvántartás"
                    >
                        <Timer size={16} />
                    </button>
                </div>
                <button
                    onClick={onDelete}
                    className="p-2 text-gray-500 hover:text-red-600 hover:bg-red-50 rounded-lg transition-colors"
                    title="Törlés"
                >
                    <Trash2 size={16} />
                </button>
            </div>
        </div>
    );
};

// Komment Modal komponens
const CommentModal = ({ task, onClose, onAddComment }) => {
    const [newComment, setNewComment] = useState('');

    const handleAddComment = () => {
        if (newComment.trim()) {
            onAddComment({
                text: newComment.trim(),
                htmlContent: newComment.trim()
            });
            setNewComment('');
        }
    };

    const formatDate = (dateString) => {
        const date = new Date(dateString);
        return date.toLocaleDateString('hu-HU', {
            year: 'numeric',
            month: 'short',
            day: 'numeric',
            hour: '2-digit',
            minute: '2-digit'
        });
    };

    return (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center p-4 z-50 backdrop-blur-sm">
            <div className="bg-white rounded-2xl max-w-3xl w-full max-h-[80vh] overflow-hidden shadow-2xl">
                <div className="p-6 border-b border-gray-200 bg-gradient-to-r from-blue-50 to-indigo-50">
                    <div className="flex items-center justify-between">
                        <div>
                            <h3 className="text-xl font-bold text-gray-800">Kommentek</h3>
                            <p className="text-gray-600 text-sm mt-1">{task.title}</p>
                        </div>
                        <button
                            onClick={onClose}
                            className="p-2 text-gray-500 hover:text-gray-700 hover:bg-gray-100 rounded-lg transition-colors"
                        >
                            <X size={24} />
                        </button>
                    </div>
                </div>

                <div className="p-6 max-h-96 overflow-y-auto">
                    {task.comments.length === 0 ? (
                        <div className="text-center py-8">
                            <MessageSquare className="w-16 h-16 text-gray-300 mx-auto mb-4" />
                            <p className="text-gray-500">Még nincsenek kommentek.</p>
                            <p className="text-gray-400 text-sm">Legyél az első, aki hozzászól!</p>
                        </div>
                    ) : (
                        <div className="space-y-4">
                            {task.comments.map((comment) => (
                                <div key={comment.id} className="bg-gray-50 rounded-xl p-4 border border-gray-200">
                                    <div className="flex items-start justify-between mb-3">
                                        <div className="flex items-center gap-3">
                                            <div className="w-8 h-8 bg-blue-500 rounded-full flex items-center justify-center">
                        <span className="text-white text-sm font-medium">
                          {comment.author.charAt(0).toUpperCase()}
                        </span>
                                            </div>
                                            <div>
                                                <span className="font-medium text-gray-800">{comment.author}</span>
                                                <div className="flex items-center gap-2 text-xs text-gray-500">
                                                    <Calendar size={12} />
                                                    <span>{formatDate(comment.timestamp)}</span>
                                                    {comment.edited && <span className="text-orange-500">(szerkesztve)</span>}
                                                </div>
                                            </div>
                                        </div>
                                    </div>
                                    <div className="text-gray-700">
                                        {comment.text}
                                    </div>
                                </div>
                            ))}
                        </div>
                    )}
                </div>

                <div className="p-6 border-t border-gray-200 bg-gray-50">
                    <div className="space-y-4">
                        <h4 className="font-medium text-gray-800">Új komment írása</h4>

                        <RichTextEditor
                            content={newComment}
                            onChange={(html, text) => setNewComment(text)}
                            placeholder="Írj egy kommentet..."
                            minHeight="100px"
                        />

                        <div className="flex justify-end">
                            <button
                                onClick={handleAddComment}
                                disabled={!newComment.trim()}
                                className="px-6 py-3 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors text-sm font-medium disabled:opacity-50 disabled:cursor-not-allowed"
                            >
                                Komment küldése
                            </button>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    );
};

// Idő nyilvántartás Modal komponens
const TimeLogModal = ({ task, onClose, onAddTimeEntry, onUpdateEstimate }) => {
    const [hours, setHours] = useState('');
    const [description, setDescription] = useState('');
    const [date, setDate] = useState(new Date().toISOString().split('T')[0]);
    const [newEstimate, setNewEstimate] = useState(task.estimatedHours);

    const handleAddTime = () => {
        if (hours && parseFloat(hours) > 0) {
            onAddTimeEntry({
                hours: parseFloat(hours),
                description: description.trim(),
                date: date
            });
            setHours('');
            setDescription('');
        }
    };

    const handleUpdateEstimate = () => {
        onUpdateEstimate(parseFloat(newEstimate) || 0);
    };

    const formatDate = (dateString) => {
        const date = new Date(dateString);
        return date.toLocaleDateString('hu-HU', {
            year: 'numeric',
            month: 'short',
            day: 'numeric'
        });
    };

    const progressPercentage = task.estimatedHours > 0 ? (task.loggedHours / task.estimatedHours) * 100 : 0;

    return (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center p-4 z-50 backdrop-blur-sm">
            <div className="bg-white rounded-2xl max-w-2xl w-full max-h-[80vh] overflow-hidden shadow-2xl">
                <div className="p-6 border-b border-gray-200 bg-gradient-to-r from-purple-50 to-pink-50">
                    <div className="flex items-center justify-between">
                        <div>
                            <h3 className="text-xl font-bold text-gray-800">Idő nyilvántartás</h3>
                            <p className="text-gray-600 text-sm mt-1">{task.title}</p>
                        </div>
                        <button
                            onClick={onClose}
                            className="p-2 text-gray-500 hover:text-gray-700 hover:bg-gray-100 rounded-lg transition-colors"
                        >
                            <X size={24} />
                        </button>
                    </div>
                </div>

                <div className="p-6 space-y-6 overflow-y-auto max-h-[calc(80vh-120px)]">
                    {/* Összegzés */}
                    <div className="bg-gradient-to-r from-blue-50 to-purple-50 rounded-xl p-4 border border-blue-200">
                        <div className="flex items-center justify-between mb-3">
                            <h4 className="font-semibold text-gray-800">Összegzés</h4>
                            <BarChart3 className="w-5 h-5 text-blue-500" />
                        </div>
                        <div className="grid grid-cols-3 gap-4 text-center">
                            <div>
                                <div className="text-2xl font-bold text-blue-600">{task.loggedHours}h</div>
                                <div className="text-sm text-gray-600">Ledolgozott</div>
                            </div>
                            <div>
                                <div className="text-2xl font-bold text-purple-600">{task.estimatedHours}h</div>
                                <div className="text-sm text-gray-600">Becsült</div>
                            </div>
                            <div>
                                <div className={`text-2xl font-bold ${progressPercentage > 100 ? 'text-red-600' : 'text-green-600'}`}>
                                    {Math.round(progressPercentage)}%
                                </div>
                                <div className="text-sm text-gray-600">Készültség</div>
                            </div>
                        </div>
                        <div className="mt-4">
                            <div className="w-full bg-gray-200 rounded-full h-3">
                                <div
                                    className={`h-3 rounded-full transition-all duration-300 ${
                                        progressPercentage > 100 ? 'bg-red-500' :
                                            progressPercentage > 80 ? 'bg-yellow-500' : 'bg-green-500'
                                    }`}
                                    style={{ width: `${Math.min(progressPercentage, 100)}%` }}
                                ></div>
                            </div>
                        </div>
                    </div>

                    {/* Becslés frissítése */}
                    <div className="bg-gray-50 rounded-xl p-4 border border-gray-200">
                        <h4 className="font-semibold text-gray-800 mb-3">Becslés frissítése</h4>
                        <div className="flex gap-3">
                            <input
                                type="number"
                                value={newEstimate}
                                onChange={(e) => setNewEstimate(e.target.value)}
                                className="flex-1 p-3 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-purple-500 focus:border-transparent"
                                placeholder="Becsült óraszám"
                                min="0"
                                step="0.5"
                            />
                            <button
                                onClick={handleUpdateEstimate}
                                className="px-6 py-3 bg-purple-600 text-white rounded-lg hover:bg-purple-700 transition-colors text-sm font-medium"
                            >
                                Frissítés
                            </button>
                        </div>
                    </div>

                    {/* Idő bejegyzés */}
                    <div className="bg-gray-50 rounded-xl p-4 border border-gray-200">
                        <h4 className="font-semibold text-gray-800 mb-3">Új idő bejegyzés</h4>
                        <div className="space-y-3">
                            <div className="grid grid-cols-2 gap-3">
                                <input
                                    type="number"
                                    value={hours}
                                    onChange={(e) => setHours(e.target.value)}
                                    className="p-3 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-purple-500 focus:border-transparent"
                                    placeholder="Órák száma"
                                    min="0"
                                    step="0.25"
                                />
                                <input
                                    type="date"
                                    value={date}
                                    onChange={(e) => setDate(e.target.value)}
                                    className="p-3 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-purple-500 focus:border-transparent"
                                />
                            </div>
                            <textarea
                                value={description}
                                onChange={(e) => setDescription(e.target.value)}
                                className="w-full p-3 border border-gray-300 rounded-lg text-sm resize-none h-20 focus:ring-2 focus:ring-purple-500 focus:border-transparent"
                                placeholder="Mit csináltál? (opcionális)"
                            />
                            <button
                                onClick={handleAddTime}
                                disabled={!hours || parseFloat(hours) <= 0}
                                className="w-full px-4 py-3 bg-purple-600 text-white rounded-lg hover:bg-purple-700 transition-colors text-sm font-medium disabled:opacity-50 disabled:cursor-not-allowed"
                            >
                                Idő hozzáadása
                            </button>
                        </div>
                    </div>

                    {/* Idő bejegyzések listája */}
                    <div>
                        <h4 className="font-semibold text-gray-800 mb-3">Korábbi bejegyzések</h4>
                        {task.timeEntries.length === 0 ? (
                            <div className="text-center py-8">
                                <Timer className="w-16 h-16 text-gray-300 mx-auto mb-4" />
                                <p className="text-gray-500">Még nincsenek idő bejegyzések.</p>
                                <p className="text-gray-400 text-sm">Add hozzá az első bejegyzésedet!</p>
                            </div>
                        ) : (
                            <div className="space-y-3 max-h-64 overflow-y-auto">
                                {task.timeEntries.map((entry) => (
                                    <div key={entry.id} className="bg-white rounded-lg p-4 border border-gray-200 shadow-sm">
                                        <div className="flex items-center justify-between mb-2">
                                            <div className="flex items-center gap-2">
                                                <Clock className="w-4 h-4 text-purple-500" />
                                                <span className="font-medium text-purple-600">{entry.hours}h</span>
                                            </div>
                                            <div className="flex items-center gap-2 text-sm text-gray-500">
                                                <Calendar size={14} />
                                                <span>{formatDate(entry.date)}</span>
                                            </div>
                                        </div>
                                        {entry.description && (
                                            <p className="text-gray-700 text-sm">{entry.description}</p>
                                        )}
                                    </div>
                                ))}
                            </div>
                        )}
                    </div>
                </div>
            </div>
        </div>
    );
};

// Test Page Export
const TestPage = () => {
    return (
        <div>
            <KanbanBoard />
        </div>
    );
};

export default TestPage;