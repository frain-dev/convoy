export interface DEVICE {
	created_at: string;
	deleted_at: string;
	host_name: string;
	last_seen_at: string;
	status: 'offline' | 'online';
	uid: string;
	updated_at: string;
}
