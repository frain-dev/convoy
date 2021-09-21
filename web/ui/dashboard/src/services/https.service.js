import * as axios from 'axios';
import { AuthDetails, APIURL } from '../helpers/get-details';

const _axios = axios.default;

const request = _axios.create({
	baseURL: APIURL,
	headers: {
		Authorization: `Bearer ${AuthDetails().token}`
	}
});

request.interceptors.response.use(
	response => {
		return response;
	},
	error => {
		if (error.response.status === 401 && error.response.config.url !== '/auth/login') window.location.replace('/login');
		return Promise.reject(error);
	}
);

export { request };
