export interface HttpResponse<T> {
	data: T;
	message: string;
	error?: any;
	status: boolean;
}

type Pagination = {
	per_page: number;
	has_next_page: boolean;
	has_prev_page: boolean;
	prev_page_cursor: string;
	next_page_cursor: string;
};

export interface PaginatedResult<T>{
	content: Array<T>
	pagination: Pagination
}
