import { ComponentFixture, TestBed } from '@angular/core/testing';
import { Router } from '@angular/router';

import { HttpService } from 'src/app/services/http/http.service';
import { LicensesService } from 'src/app/services/licenses/licenses.service';
import { RbacService } from 'src/app/services/rbac/rbac.service';

import { QueueMonitoringAsynqmonComponent } from './queue-monitoring-asynqmon.component';

const UNLICENSED_COPY = 'Your license does not include Asynq monitoring';

describe('QueueMonitoringAsynqmonComponent', () => {
	let fixture: ComponentFixture<QueueMonitoringAsynqmonComponent>;
	let licensed: boolean;
	let answerRole!: () => void;
	let answerLicenses!: () => void;

	beforeEach(async () => {
		licensed = false;

		await TestBed.configureTestingModule({
			imports: [QueueMonitoringAsynqmonComponent],
			providers: [
				{
					provide: LicensesService,
					useValue: {
						// Held open so a test can paint the page while the read is
						// still in flight. hasInstanceLicense answers off a cache this
						// component refreshes first, so that window is every visit.
						loadAllLicenses: () => new Promise<void>(resolve => (answerLicenses = resolve as () => void)),
						hasInstanceLicense: () => licensed
					}
				},
				{ provide: RbacService, useValue: { getUserRole: () => new Promise(resolve => (answerRole = () => resolve('INSTANCE_ADMIN'))) } },
				// No token, so the licensed path stops at the session mint instead of
				// reaching for the network. The banner is the subject here, not the
				// iframe behind it.
				{ provide: HttpService, useValue: { authDetails: () => null } },
				{ provide: Router, useValue: { navigateByUrl: () => {} } }
			]
		}).compileComponents();

		fixture = TestBed.createComponent(QueueMonitoringAsynqmonComponent);

		// The first pass runs ngOnInit, so ngOnInit is never called by hand here:
		// a second call would start a second pair of reads and overwrite the
		// resolvers the test is holding.
		fixture.detectChanges();
	});

	afterEach(() => {
		fixture.destroy();
	});

	// The component's own detector, not fixture.detectChanges: these tests move
	// state between passes on purpose, which is what the fixture's verification
	// pass exists to reject.
	function paint() {
		fixture.changeDetectorRef.detectChanges();
	}

	function text(): string {
		return (fixture.nativeElement as HTMLElement).textContent ?? '';
	}

	function loader(): Element | null {
		return (fixture.nativeElement as HTMLElement).querySelector('convoy-loader');
	}

	// Found by copy rather than position: the footer's other button is the
	// fullscreen toggle, which has its own disabled rule.
	function refreshButton(): HTMLButtonElement | undefined {
		const buttons = Array.from((fixture.nativeElement as HTMLElement).querySelectorAll('button'));
		return buttons.find(button => /Retry|Refresh/.test(button.textContent ?? ''));
	}

	async function settle() {
		for (let turn = 0; turn < 5; turn++) await Promise.resolve();
	}

	it('does not claim the license excludes monitoring before the role read lands', () => {
		paint();

		expect(text()).not.toContain(UNLICENSED_COPY);
	});

	it('does not claim it while the license read is in flight', async () => {
		answerRole();
		await settle();

		paint();

		expect(text()).not.toContain(UNLICENSED_COPY);
	});

	it('claims it once the license read answers that monitoring is excluded', async () => {
		answerRole();
		await settle();
		answerLicenses();
		await settle();

		paint();

		expect(text()).toContain(UNLICENSED_COPY);
	});

	it('never claims it when the license includes monitoring', async () => {
		licensed = true;

		answerRole();
		await settle();
		answerLicenses();
		await settle();

		paint();

		expect(text()).not.toContain(UNLICENSED_COPY);
	});

	// This page only mounts under the parent's licensed gate, so the cache it
	// reads is already warm and the card is on screen from the first pass. Its
	// body is what has nothing in it until the mint starts.
	it('shows a loader in the card while the reads before the mint are in flight', () => {
		licensed = true;

		paint();

		expect(loader()).not.toBeNull();
	});

	// A click here mints a session that ngOnInit's own mint then discards, so the
	// control has to stay dead for as long as the loader is up.
	it('keeps the refresh button disabled while that loader is up', () => {
		licensed = true;

		paint();

		expect(refreshButton()?.disabled).toBeTrue();
	});

	it('drops the loader once the reads resolve and the mint reports', async () => {
		licensed = true;

		answerRole();
		await settle();
		answerLicenses();
		await settle();

		paint();

		expect(loader()).toBeNull();
	});

	it('shows no loader once the license read answers that monitoring is excluded', async () => {
		answerRole();
		await settle();
		answerLicenses();
		await settle();

		paint();

		expect(loader()).toBeNull();
		expect(text()).toContain(UNLICENSED_COPY);
	});
});
