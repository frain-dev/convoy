import { ComponentFixture, TestBed } from '@angular/core/testing';
import { CommonModule } from '@angular/common';
import { TableIndexesComponent } from './table-indexes.component';
import { RunCardComponent } from '../runs/run-card.component';
import { MaintenanceRun } from '../runs/run.model';
import { TagComponent } from 'src/app/components/tag/tag.component';
import { StatusColorModule } from 'src/app/pipes/status-color/status-color.module';
import { AdminService } from '../admin.service';
import { GeneralService } from 'src/app/services/general/general.service';

const DROPPED = [
	{ table: 'event_deliveries', name: 'idx_event_deliveries_usage', dropped_at: '2026-08-19T09:00:00Z', unique: false },
	{ table: 'events', name: 'idx_events_source_id', dropped_at: '2026-08-19T09:00:00Z', unique: true }
];

function runningRebuild(): MaintenanceRun {
	return {
		uid: 'run-1',
		table_name: 'events',
		operation: 'rebuild_index',
		index_name: 'idx_events_source_id',
		status: 'running',
		phase: null,
		steps: null,
		error: null,
		triggered_by: 'user-1',
		started_at: new Date().toISOString(),
		updated_at: new Date().toISOString(),
		completed_at: null
	};
}

describe('TableIndexesComponent', () => {
	let fixture: ComponentFixture<TableIndexesComponent>;
	let component: TableIndexesComponent;
	let started: string[];

	beforeEach(async () => {
		started = [];

		await TestBed.configureTestingModule({
			declarations: [TableIndexesComponent, RunCardComponent],
			imports: [CommonModule, TagComponent, StatusColorModule],
			providers: [
				// Left pending on purpose. ngOnInit still runs, and a stub that
				// resolves would land mid-pass and re-render the page from a
				// half-loaded state while these tests are reading it.
				{
					provide: AdminService,
					useValue: {
						listIndexes: () => new Promise(() => {}),
						listPartitionRuns: () => new Promise(() => {}),
						startIndexRebuild: (request: { index: string }) => {
							started.push(request.index);
							return Promise.resolve({ data: {} });
						}
					}
				},
				{ provide: GeneralService, useValue: { showNotification: () => {} } }
			]
		}).compileComponents();

		fixture = TestBed.createComponent(TableIndexesComponent);
		component = fixture.componentInstance;
	});

	// Seeded before the first render, not after it. The index report resolves
	// across change detection passes, and a write landing between two passes is
	// reported as a change after the view was checked rather than as new state.
	async function render(seed: Partial<TableIndexesComponent> = {}) {
		// runsKnown defaults on because a read that answered with no open run is the
		// ordinary state, and the buttons are gated on it. Tests about the unknown
		// state turn it back off.
		Object.assign(component, { dropped: DROPPED, reportKnown: true, runsKnown: true }, seed);

		// Angular schedules the render, as it does for a real click. Driving
		// detectChanges by hand instead leaves the view unrefreshed, because
		// nothing in the test marked it dirty.
		fixture.autoDetectChanges();
		await fixture.whenStable();
	}

	function rebuildButtons(): HTMLButtonElement[] {
		return Array.from(fixture.nativeElement.querySelectorAll('button')).filter(button => (button as HTMLButtonElement).textContent?.trim() === 'Rebuild') as HTMLButtonElement[];
	}

	// A missing unique index is not just slower, it is not enforcing its key, so
	// the page has to say so rather than listing it beside the rest.
	it('names the cost of a missing unique index', async () => {
		await render();

		const text = fixture.nativeElement.textContent as string;
		expect(text).toContain('Nothing stops a duplicate row on one of these');
		expect(component.costs(DROPPED[1])).toContain('no longer unique');
		expect(component.costs(DROPPED[0])).toBe('Slower queries');
	});

	it('rebuilds the index whose button was pressed', async () => {
		await render();

		rebuildButtons()[0].click();
		await fixture.whenStable();

		expect(started).toEqual(['idx_event_deliveries_usage']);
	});

	// The server allows one run at a time for the whole instance, conversions
	// included, so a button offered while one is in flight would only produce a
	// 409 the operator has to read.
	it('offers no rebuild while another run holds the instance', async () => {
		await render({ runs: [runningRebuild()] });

		expect(component.canStart).toBeFalse();
		expect(rebuildButtons().every(button => button.disabled)).toBeTrue();
	});

	// An empty report after a failed request is unknown, not an all-clear. Showing
	// the all-clear would tell an operator their indexes are fine because a
	// request failed.
	it('does not read a failed report as nothing to rebuild', async () => {
		await render({ dropped: [], reportKnown: false, reportFailed: true });

		const text = fixture.nativeElement.textContent as string;
		expect(text).toContain('could not be loaded');
		expect(text).not.toContain('Nothing is waiting to be rebuilt');
	});

	// The state every operator sees first: nothing has answered yet. Claiming the
	// read failed is as wrong as claiming it came back clean, and the runs read
	// has not even been made at this point, since it follows the report.
	it('claims neither an all-clear nor a failure before the reads answer', async () => {
		await render({ dropped: [], reportKnown: false, runs: [], runsKnown: false });

		const text = fixture.nativeElement.textContent as string;
		expect(text).toContain('Reading the index report');
		expect(text).toContain('Reading the rebuild history');
		expect(text).not.toContain('could not be loaded');
		expect(text).not.toContain('Nothing is waiting to be rebuilt');
		expect(text).not.toContain('No rebuilds have been run');
	});

	// A failed history read leaves the operator unable to see a run in flight, so
	// it must not be reported as an instance that has never run one.
	it('does not read a failed history as an instance that never rebuilt', async () => {
		await render({ runs: [], runsKnown: false, runsFailed: true });

		const text = fixture.nativeElement.textContent as string;
		expect(text).toContain('The rebuild history could not be loaded');
		expect(text).not.toContain('No rebuilds have been run');
	});

	// The same unknown state, now as a gate rather than as copy. The slot is
	// instance-wide and a conversion started on the other page or from a shell
	// takes it, so a stale empty list is not evidence that it is free.
	it('offers no rebuild while the run history is unknown', async () => {
		await render({ runs: [], runsKnown: false, runsFailed: true });

		expect(component.canStart).toBeFalse();
		expect(rebuildButtons().length).toBeGreaterThan(0);
		expect(rebuildButtons().every(button => button.disabled)).toBeTrue();
	});
});
