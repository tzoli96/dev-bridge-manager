import axios, { AxiosInstance, AxiosResponse, AxiosError } from 'axios'

// Environment-aware base URL
const getBaseURL = (): string => {
    // Browser environment
    if (typeof window !== 'undefined') {
        return process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080/api/v1'
    }

    // Server-side (Docker network)
    return process.env.INTERNAL_API_URL || 'http://backend:8080/api/v1'
}

class ApiClient {
    private client: AxiosInstance

    constructor() {
        this.client = axios.create({
            baseURL: getBaseURL(),
            timeout: 10000,
            headers: {
                'Content-Type': 'application/json',
            },
        })

        // Request interceptor
        this.client.interceptors.request.use(
            (config) => {
                // Add auth token if available
                // const token = localStorage.getItem('token')
                // if (token) {
                //   config.headers.Authorization = `Bearer ${token}`
                // }
                return config
            },
            (error) => Promise.reject(error)
        )

        // Response interceptor
        this.client.interceptors.response.use(
            (response: AxiosResponse) => response,
            (error: AxiosError) => {
                if (error.response?.status === 401) {
                    // Handle unauthorized
                    // logout() or redirect to login
                }

                // Better error handling
                const message = error.response?.data?.message || error.message || 'Something went wrong'
                return Promise.reject(new Error(message))
            }
        )
    }

    // GET request
    async get<T>(endpoint: string, params?: any): Promise<T> {
        const response = await this.client.get<T>(endpoint, { params })
        return response.data
    }

    // POST request
    async post<T>(endpoint: string, data?: any): Promise<T> {
        const response = await this.client.post<T>(endpoint, data)
        return response.data
    }

    // PUT request
    async put<T>(endpoint: string, data?: any): Promise<T> {
        const response = await this.client.put<T>(endpoint, data)
        return response.data
    }

    // DELETE request
    async delete<T>(endpoint: string): Promise<T> {
        const response = await this.client.delete<T>(endpoint)
        return response.data
    }

    // PATCH request
    async patch<T>(endpoint: string, data?: any): Promise<T> {
        const response = await this.client.patch<T>(endpoint, data)
        return response.data
    }

    // File upload
    async upload<T>(endpoint: string, file: File, onProgress?: (progress: number) => void): Promise<T> {
        const formData = new FormData()
        formData.append('file', file)

        const response = await this.client.post<T>(endpoint, formData, {
            headers: {
                'Content-Type': 'multipart/form-data',
            },
            onUploadProgress: (progressEvent) => {
                if (onProgress && progressEvent.total) {
                    const progress = Math.round((progressEvent.loaded * 100) / progressEvent.total)
                    onProgress(progress)
                }
            },
        })

        return response.data
    }

    // Health check
    async healthCheck() {
        return this.get('/health')
    }

    // Set auth token
    setAuthToken(token: string) {
        this.client.defaults.headers.common['Authorization'] = `Bearer ${token}`
    }

    // Remove auth token
    removeAuthToken() {
        delete this.client.defaults.headers.common['Authorization']
    }
}

export const apiClient = new ApiClient()

// Export types for convenience
export type ApiError = AxiosError
export type { AxiosResponse }