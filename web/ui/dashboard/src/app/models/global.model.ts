export interface PAGINATION {
	next: number;
	page: number;
	perPage: number;
	prev: number;
	total: number;
	totalPage: number;
}

export type STATUS_COLOR = 'grey' | 'success' | 'warning' | 'danger';
