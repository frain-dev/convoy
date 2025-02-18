import convoyLogo from './assets/img/svg/convoy-logo-full-new.svg';
import { Button } from './components/ui/button';

function App() {
	return (
		<div className="flex w-full">
			<div className="bg-white py-24 mx-auto text-center">
				<img src={convoyLogo} alt="convoy" className="mx-auto py-6" />
				<h2 className="text-neutral py-4 tracking-tight text-pretty text-16 font-semibold">
					The complete solution for secure, scalable, and reliable webhook delivery.
				</h2>
				<div className="pt-4">
					<Button size="lg">Learn more</Button>
				</div>
			</div>
		</div>
	);
}

export default App;
