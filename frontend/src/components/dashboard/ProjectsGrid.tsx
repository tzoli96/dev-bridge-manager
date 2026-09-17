import { User } from '@/types/user'
import { Project } from '@/services/projectsService'
import { isAdmin } from '@/utils/permissions'
import { cn } from '@/lib/utils'
import { useState, useEffect } from 'react'
import { Users } from 'lucide-react'

interface ProjectsGridProps {
    projects: Project[]
    user: User | null
    onEditProject: (project: Project) => void
    onDeleteProject: (projectId: number) => void
    onManageTeam: (project: Project) => void
    deleting: number | null
}

export default function ProjectsGrid({
    projects,
    user,
    onEditProject,
    onDeleteProject,
    onManageTeam,
    deleting
}: ProjectsGridProps) {
    const [visibleProjects, setVisibleProjects] = useState<number[]>([]);
    const [activeCard, setActiveCard] = useState<number | null>(null);

    // Staggered entrance animation
    useEffect(() => {
        const timer = setTimeout(() => {
            const projectIds = projects.map(p => p.id);
            setVisibleProjects(projectIds);
        }, 100);
        
        return () => clearTimeout(timer);
    }, [projects]);

    const getStatusColor = (status: string) => {
        switch (status) {
            case 'active': return 'bg-success/10 text-success hover:bg-success/20'
            case 'completed': return 'bg-primary/10 text-primary hover:bg-primary/20'
            case 'on-hold': return 'bg-warning/10 text-warning hover:bg-warning/20'
            default: return 'bg-destructive/10 text-destructive hover:bg-destructive/20'
        }
    }

    const handleCardMouseDown = (projectId: number) => {
        setActiveCard(projectId);
    };

    const handleCardMouseUp = () => {
        setActiveCard(null);
    };

    return (
        <div className="space-y-4">
            <div className="bg-muted px-6 py-3 rounded-lg">
                <p className="text-sm text-muted-foreground">
                    Showing {projects.length} project{projects.length !== 1 ? 's' : ''}
                </p>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
                {projects.map((project, index) => (
                    <div 
                        key={project.id}
                        className={cn(
                            "bg-card rounded-xl p-6 relative border border-border",
                            "transition-all duration-300 ease-in-out",
                            "hover:shadow-sm hover:translate-y-[-4px]",
                            activeCard === project.id && "shadow-inner translate-y-[2px]",
                            visibleProjects.includes(project.id) 
                                ? "opacity-100 transform translate-y-0" 
                                : "opacity-0 transform translate-y-8"
                        )}
                        style={{ 
                            transitionDelay: `${index * 50}ms`,
                            boxShadow: "0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -1px rgba(0, 0, 0, 0.06)"
                        }}
                        onMouseDown={() => handleCardMouseDown(project.id)}
                        onMouseUp={handleCardMouseUp}
                        onMouseLeave={handleCardMouseUp}
                    >
                        <div className="flex justify-between items-start mb-3">
                            <h3 className="text-lg font-semibold text-foreground truncate group-hover:text-primary transition-colors">
                                {project.name}
                            </h3>
                            <span className={cn(
                                "inline-flex px-2 py-1 text-xs font-semibold rounded-full",
                                "transition-all duration-200 ease-in-out",
                                getStatusColor(project.status)
                            )}>
                                {project.status}
                            </span>
                        </div>

                        {project.description && (
                            <p className="text-muted-foreground text-sm mb-4 line-clamp-3">
                                {project.description}
                            </p>
                        )}

                        <div className="text-xs text-muted-foreground mb-4">
                            <div>Created by: <span className="font-medium">{project.created_by_name}</span></div>
                            <div>Created: <span className="font-medium">{new Date(project.created_at).toLocaleDateString()}</span></div>
                        </div>

                        <div className="flex flex-col space-y-2 pt-4 border-t">
                            {/* Team Management - Available for all users */}
                            <button
                                onClick={() => onManageTeam(project)}
                                className={cn(
                                    "flex items-center gap-1.5 text-primary text-sm font-medium text-left",
                                    "transition-all duration-200 ease-in-out",
                                    "hover:text-primary hover:pl-1",
                                    "active:text-primary active:scale-[0.98]",
                                    "focus:outline-none focus:ring-2 focus:ring-ring focus:ring-opacity-50 rounded"
                                )}
                            >
                                <Users size={14} />
                                Manage Team
                            </button>

                            {/* Admin-only actions */}
                            {isAdmin(user) && (
                                <div className="flex space-x-2 pt-2 border-t">
                                    <button
                                        onClick={() => onEditProject(project)}
                                        className={cn(
                                            "flex-1 text-primary text-sm font-medium",
                                            "transition-all duration-200 ease-in-out",
                                            "hover:text-primary hover:bg-primary/10 p-1 rounded",
                                            "active:text-primary active:scale-[0.98]",
                                            "focus:outline-none focus:ring-2 focus:ring-ring focus:ring-opacity-50"
                                        )}
                                    >
                                        Edit
                                    </button>
                                    <button
                                        onClick={() => onDeleteProject(project.id)}
                                        disabled={deleting === project.id}
                                        className={cn(
                                            "flex-1 text-destructive text-sm font-medium",
                                            "transition-all duration-200 ease-in-out",
                                            "hover:text-destructive hover:bg-destructive/10 p-1 rounded",
                                            "active:text-destructive active:scale-[0.98]",
                                            "focus:outline-none focus:ring-2 focus:ring-destructive/40 focus:ring-opacity-50",
                                            "disabled:opacity-50 disabled:cursor-not-allowed disabled:hover:bg-transparent disabled:hover:text-destructive disabled:hover:scale-100"
                                        )}
                                    >
                                        {deleting === project.id ? (
                                            <span className="inline-flex items-center">
                                                <svg className="animate-spin -ml-1 mr-2 h-4 w-4 text-destructive" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                                                    <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                                                    <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                                                </svg>
                                                Deleting...
                                            </span>
                                        ) : 'Delete'}
                                    </button>
                                </div>
                            )}
                        </div>
                        
                        {/* Hover effect overlay */}
                        <div className="absolute inset-0 rounded-lg pointer-events-none transition-opacity duration-300 opacity-0 hover:opacity-100">
                            <div className="absolute inset-0 bg-gradient-to-t from-primary/10 to-transparent opacity-20 rounded-lg"></div>
                        </div>
                    </div>
                ))}
            </div>
        </div>
    )
}