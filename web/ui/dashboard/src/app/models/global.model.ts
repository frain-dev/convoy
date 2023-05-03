export interface PAGINATION {
	next: number;
	page: number;
	perPage: number;
	prev: number;
	total: number;
	totalPage: number;
	has_next_page: boolean;
	has_prev_page: boolean;
	next_page_cursor: string;
	per_page: number;
	prev_page_cursor: string;
}
export interface CHARTDATA {
	label: string;
	data: number;
}

export type STATUS_COLOR = 'grey' | 'success' | 'warning' | 'danger';

export type NOTIFICATION_STATUS = 'warning' | 'info' | 'success' | 'error';

export interface CURSOR {
	next_page_cursor?: string;
	prev_page_cursor?: string;
	direction?: 'next' | 'prev';
}

export interface HTTP_RESPONSE {
	data: any;
	message: string;
	error?: any;
	status: boolean;
}
