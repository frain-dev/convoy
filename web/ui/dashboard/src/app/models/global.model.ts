export interface PAGINATION {
	next: number;
	page: number;
	perPage: number;
	prev: number;
	total: number;
	totalPage: number;
}
export interface CHARTDATA {
	label: string;
	data: number;
}

export type STATUS_COLOR = 'grey' | 'success' | 'warning' | 'danger';

export type NOTIFICATION_STATUS = 'warning' | 'info' | 'success' | 'error';
