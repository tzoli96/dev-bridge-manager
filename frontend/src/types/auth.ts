export interface User {
    id: number;
    name: string;
    email: string;
    position: string;
    role: {
        id: number;
        name: string;
        display_name: string;
    };
    permissions: string[];
}

export interface AuthContextType {
    user: User | null;
    token: string | null;
    login: (email: string, password: string) => Promise<void>;
    register: (userData: RegisterRequest) => Promise<void>;
    logout: () => void;
    loading: boolean;
    isAuthenticated: boolean;
    hasPermission: (permission: string) => boolean;
    hasAnyPermission: (permissions: string[]) => boolean;
    hasRole: (role: string) => boolean;
    canAccess: (requiredPermissions: string | string[]) => boolean;
    setUser: (user: User) => void;
}

export interface AuthResponse {
    success: boolean
    message: string
    token?: string
    user?: User
}

export interface LoginRequest {
    email: string
    password: string
}

export interface RegisterRequest {
    name: string
    email: string
    password: string
    position: string
}