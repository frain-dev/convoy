import { ComponentFixture, TestBed } from '@angular/core/testing';
import { CommonModule } from '@angular/common';
import { RetentionDropsComponent } from './retention-drops.component';
import { RetentionRunCardComponent } from '../runs/retention-run-card.component';
import { AdminService } from '../admin.service';
import { LicensesService } from 'src/app/services/licenses/licenses.service';

describe('RetentionDropsComponent', () => {
	let fixture: ComponentFixture<RetentionDropsComponent>;

	beforeEach(async () => {
		await TestBed.configureTestingModule({
			declarations: [RetentionDropsComponent, RetentionRunCardComponent],
			imports: [CommonModule],
			providers: [
				{
					provide: AdminService,
					useValue: {
						listRetentionRuns: () => new Promise(() => {})
					}
				},
				{
					provide: LicensesService,
					useValue: {
						loadAllLicenses: () => Promise.resolve(),
						hasInstanceLicense: () => true
					}
				}
			]
		}).compileComponents();

		fixture = TestBed.createComponent(RetentionDropsComponent);
	});

	it('shows the licensed empty state while history is loading', () => {
		const component = fixture.componentInstance;
		// Seeded rather than awaited through ngOnInit: license and run fetches
		// resolve across change detection passes, and half-loaded renders are not
		// what this test is about.
		component.hasRetentionLicense = true;
		component.licenseKnown = true;
		fixture.detectChanges();

		const copy = fixture.nativeElement.textContent;
		expect(copy).toContain('Retention drops');
		expect(copy).toContain('Reading retention drop history');
	});
});
