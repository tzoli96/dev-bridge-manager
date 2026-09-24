// frontend/src/services/jobSearchService.ts
import { apiClient } from '@/lib/api'

export interface JobSearchProfile {
    id: number
    cv_text: string
    skills: string
    preferences: string
    updated_by: number
    updated_at: string
}

export interface JobListing {
    id: number
    site: string
    external_url: string
    title: string
    company: string
    location: string
    description: string
    posted_at: string | null
    scraped_at: string
}

export interface JobMatch {
    id: number
    job_listing_id: number
    job_listing: JobListing
    score: number
    reasoning: string
    status: 'new' | 'reviewed' | 'dismissed' | 'applied'
    applied_at: string | null
    application_text: string | null
    created_at: string
    updated_at: string
}

export interface ManualListingInput {
    url: string
    title?: string
    company?: string
    location?: string
    description?: string
}

interface ProfileApiResponse {
    success: boolean
    message?: string
    profile?: JobSearchProfile
}

interface MatchesApiResponse {
    success: boolean
    message?: string
    matches?: JobMatch[]
}

interface ScanNowApiResponse {
    success: boolean
    message?: string
    new_listings?: number
    new_matches?: number
}

interface ManualAddApiResponse {
    success: boolean
    message?: string
    match?: JobMatch
}

interface DraftApiResponse {
    success: boolean
    message?: string
    draft?: string
}

export const JobSearchService = {
    async getProfile(): Promise<JobSearchProfile> {
        const response = await apiClient.get<ProfileApiResponse>('/admin/job-search/profile')
        if (response.success && response.profile) return response.profile
        throw new Error(response.message || 'Failed to fetch job search profile')
    },

    async updateProfile(data: { cv_text: string; skills: string; preferences: string }): Promise<JobSearchProfile> {
        const response = await apiClient.put<ProfileApiResponse>('/admin/job-search/profile', data)
        if (response.success && response.profile) return response.profile
        throw new Error(response.message || 'Failed to update job search profile')
    },

    async scanNow(): Promise<{ newListings: number; newMatches: number }> {
        // Scan-now runs the scraper and scores every new listing
        // synchronously - same reasoning as draftReply's longer timeout,
        // just larger since this can score many listings in one call.
        const response = await apiClient.post<ScanNowApiResponse>('/admin/job-search/scan-now', {}, 40000)
        if (response.success) {
            return { newListings: response.new_listings || 0, newMatches: response.new_matches || 0 }
        }
        throw new Error(response.message || 'Failed to run scan')
    },

    async addManualListing(input: ManualListingInput): Promise<JobMatch> {
        const response = await apiClient.post<ManualAddApiResponse>('/admin/job-search/listings/manual', input, 40000)
        if (response.success && response.match) return response.match
        throw new Error(response.message || 'Failed to add listing')
    },

    async listMatches(status?: string): Promise<JobMatch[]> {
        const response = await apiClient.get<MatchesApiResponse>('/admin/job-search/matches', status ? { status } : undefined)
        if (response.success && response.matches) return response.matches
        throw new Error(response.message || 'Failed to fetch matches')
    },

    async updateMatchStatus(id: number, status: 'reviewed' | 'dismissed'): Promise<void> {
        const response = await apiClient.patch<{ success: boolean; message?: string }>(`/admin/job-search/matches/${id}/status`, { status })
        if (!response.success) throw new Error(response.message || 'Failed to update match status')
    },

    async draftApplication(id: number, instruction?: string): Promise<string> {
        const response = await apiClient.post<DraftApiResponse>(`/admin/job-search/matches/${id}/draft-application`, { instruction }, 40000)
        if (response.success && response.draft !== undefined) return response.draft
        throw new Error(response.message || 'Failed to draft application')
    },

    async markApplied(id: number, applicationText: string): Promise<void> {
        const response = await apiClient.post<{ success: boolean; message?: string }>(`/admin/job-search/matches/${id}/mark-applied`, { application_text: applicationText })
        if (!response.success) throw new Error(response.message || 'Failed to mark as applied')
    },
}
