import moment from 'moment';
import Vue from 'vue';

Vue.filter('date', function (value) {
	console.log('🚀 ~ file: date.js ~ line 5 ~ value', value);
	if (value) {
		return moment(String(value)).format('MMMM DD, YYYY');
	}
});
