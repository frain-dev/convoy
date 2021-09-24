import './app.scss';
import { DashboardPage } from './pages/dashboard';
import { LoginPage } from './pages/login';
import { BrowserRouter as Router, Route, Redirect } from 'react-router-dom';
import { AuthDetails } from './helpers/get-details';

export default function App() {
	return (
		<Router>
			<Route exact path="/" render={() => (AuthDetails().authState ? <DashboardPage /> : <Redirect to="/login" />)} />
			<Route exact path="/login" component={LoginPage} />
		</Router>
	);
}
