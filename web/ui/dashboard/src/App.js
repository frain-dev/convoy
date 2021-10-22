import './app.scss';
import { DashboardPage } from './pages/dashboard';
import { LoginPage } from './pages/login';
import { BrowserRouter as Router, Route } from 'react-router-dom';

export default function App() {
	return (
		<Router>
			<Route exact path="/" component={DashboardPage} />
			<Route exact path="/login" component={LoginPage} />
		</Router>
	);
}
