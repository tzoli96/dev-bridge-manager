'use client';

import React, { useState } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { ProgressBar } from '@/components/ui/progress-bar';
import { useTimeEntries, useTasks } from '@/hooks/kanban';
import { formatDistanceToNow } from 'date-fns';
import { hu } from 'date-fns/locale';
import {
    Timer,
    Plus,
    Edit2,
    Trash2,
    Clock,
    Calendar,
    BarChart3
} from 'lucide-react';

interface TimeTrackerProps {
    taskId: string;
}

export const TimeTracker: React.FC<TimeTrackerProps> = ({ taskId }) => {
    const { getTask } = useTasks('');
    const {
        getEntriesByTask,
        getTotalHoursByTask,
        addTimeEntry,
        updateTaskEstimate,
        isLoading
    } = useTimeEntries('');

    const task = getTask(taskId);
    const timeEntries = getEntriesByTask(taskId);
    const totalHours = getTotalHoursByTask(taskId);

    const [newEntry, setNewEntry] = useState({
        hours: 0,
        description: '',
        date: new Date().toISOString().split('T')[0]
    });
    const [newEstimate, setNewEstimate] = useState(task?.estimatedHours || 0);

    if (!task) return null;

    const progressPercentage = task.estimatedHours > 0
        ? (totalHours / task.estimatedHours) * 100
        : 0;

    const handleAddTimeEntry = async () => {
        if (newEntry.hours <= 0) return;

        try {
            await addTimeEntry(taskId, newEntry);
            setNewEntry({ hours: 0, description: '', date: new Date().toISOString().split('T')[0] });
        } catch (error) {
            console.error('Error adding time entry:', error);
        }
    };

    const handleUpdateEstimate = async () => {
        try {
            await updateTaskEstimate(taskId, newEstimate);
        } catch (error) {
            console.error('Error updating estimate:', error);
        }
    };

    return (
        <div className="p-6 space-y-6">
            {/* Summary */}
            <div className="bg-gradient-to-r from-blue-50 to-purple-50 rounded-lg p-4 border">
                <div className="flex items-center justify-between mb-3">
                    <h4 className="font-semibold text-gray-900">Time Summary</h4>
                    <BarChart3 className="w-5 h-5 text-blue-600" />
                </div>

                <div className="grid grid-cols-3 gap-4 text-center mb-4">
                    <div>
                        <div className="text-2xl font-bold text-blue-600">{totalHours}h</div>
                        <div className="text-sm text-gray-600">Logged</div>
                    </div>
                    <div>
                        <div className="text-2xl font-bold text-purple-600">{task.estimatedHours}h</div>
                        <div className="text-sm text-gray-600">Estimated</div>
                    </div>
                    <div>
                        <div className={`text-2xl font-bold ${
                            progressPercentage > 100 ? 'text-red-600' : 'text-green-600'
                        }`}>
                            {Math.round(progressPercentage)}%
                        </div>
                        <div className="text-sm text-gray-600">Progress</div>
                    </div>
                </div>

                <ProgressBar
                    percentage={progressPercentage}
                    variant={progressPercentage > 100 ? 'danger' : 'default'}
                    showLabel
                />
            </div>

            {/* Update Estimate */}
            <div className="bg-gray-50 rounded-lg p-4">
                <h4 className="font-semibold text-gray-900 mb-3">Update Estimate</h4>
                <div className="flex gap-3">
                    <Input
                        type="number"
                        value={newEstimate}
                        onChange={(value) => setNewEstimate(parseFloat(value) || 0)}
                        placeholder="Estimated hours"
                        min="0"
                        step="0.5"
                        className="flex-1"
                    />
                    <Button onClick={handleUpdateEstimate}>
                        Update
                    </Button>
                </div>
            </div>

            {/* Add Time Entry */}
            <div className="bg-gray-50 rounded-lg p-4">
                <h4 className="font-semibold text-gray-900 mb-3">Log Time</h4>
                <div className="space-y-3">
                    <div className="grid grid-cols-2 gap-3">
                        <Input
                            label="Hours"
                            type="number"
                            value={newEntry.hours}
                            onChange={(value) => setNewEntry(prev => ({
                                ...prev,
                                hours: parseFloat(value) || 0
                            }))}
                            placeholder="0"
                            min="0"
                            step="0.25"
                        />
                        <Input
                            label="Date"
                            type="date"
                            value={newEntry.date}
                            onChange={(value) => setNewEntry(prev => ({ ...prev, date: value }))}
                        />
                    </div>

                    <Input
                        label="Description"
                        value={newEntry.description}
                        onChange={(value) => setNewEntry(prev => ({ ...prev, description: value }))}
                        placeholder="What did you work on?"
                    />

                    <Button
                        onClick={handleAddTimeEntry}
                        disabled={newEntry.hours <= 0 || isLoading}
                        loading={isLoading}
                        icon={Plus}
                        className="w-full"
                    >
                        Log Time
                    </Button>
                </div>
            </div>

            {/* Time Entries List */}
            <div>
                <h4 className="font-semibold text-gray-900 mb-3">Time Entries</h4>
                {timeEntries.length === 0 ? (
                    <div className="text-center py-8 text-gray-500">
                        <Timer className="w-12 h-12 mx-auto mb-3 text-gray-300" />
                        <p>No time entries yet.</p>
                    </div>
                ) : (
                    <div className="space-y-3 max-h-64 overflow-y-auto">
                        {timeEntries.map((entry) => (
                            <div key={entry.id} className="bg-white rounded-lg p-3 border">
                                <div className="flex items-center justify-between mb-2">
                                    <div className="flex items-center gap-2">
                                        <Clock className="w-4 h-4 text-purple-600" />
                                        <span className="font-medium text-purple-600">{entry.hours}h</span>
                                    </div>
                                    <div className="flex items-center gap-2 text-sm text-gray-500">
                                        <Calendar className="w-4 h-4" />
                                        <span>{new Date(entry.date).toLocaleDateString('hu-HU')}</span>
                                    </div>
                                </div>
                                {entry.description && (
                                    <p className="text-sm text-gray-700">{entry.description}</p>
                                )}
                                <p className="text-xs text-gray-500 mt-1">
                                    by {entry.user?.name || 'Unknown'} • {
                                    formatDistanceToNow(new Date(entry.createdAt), {
                                        addSuffix: true,
                                        locale: hu
                                    })
                                }
                                </p>
                            </div>
                        ))}
                    </div>
                )}
            </div>
        </div>
    );
};