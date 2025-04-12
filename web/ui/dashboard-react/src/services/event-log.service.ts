import { request } from './http.service';

type RetryCountParams = {
    endpointId?: string;
    page?: number;
    startDate?: string;
    endDate?: string;
    sourceId?: string;
};

type BatchRetryParams = {
    startDate?: string;
    endDate?: string;
    endpointId?: string;
    sourceId?: string;
};

/**
 * Get count of events that would be affected by a batch retry
 */
export async function getRetryCount(requestDetails: RetryCountParams) {
    const res = await request<{ count: number }>({
        url: `/events/countbatchreplayevents`,
        method: 'get',
        level: 'org_project',
        query: requestDetails
    });

    return res.data;
}

/**
 * Retry a single event
 */
export async function retryEvent(eventId: string) {
    const res = await request({
        url: `/events/${eventId}/replay`,
        method: 'put',
        level: 'org_project'
    });

    return res.data;
}

/**
 * Batch retry events based on filter criteria
 */
export async function batchRetryEvent(requestDetails: BatchRetryParams) {
    const res = await request({
        url: `/events/batchreplay`,
        method: 'post',
        body: null,
        level: 'org_project',
        query: requestDetails
    });

    return res.data;
} 
