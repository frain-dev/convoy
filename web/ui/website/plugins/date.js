import moment from 'moment';
import Vue from 'vue';

Vue.filter('date', function (value) {
	if (value) {
		return moment(String(value)).format('MMMM DD, YYYY');
	}
});
