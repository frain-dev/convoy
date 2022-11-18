import { Story, Meta } from '@storybook/angular/types-6-0';
import { TableCellComponent } from '../../app/components/table-cell/table-cell.component';

export default {
	title: 'Example/TableCell',
	component: TableCellComponent,
	argTypes: {
		forDate: {
			control: { type: 'boolean' }
		}
	}
} as Meta;

const Template: Story<TableCellComponent> = (args: TableCellComponent) => ({
	props: args,
	template: `<td convoy-table-cell [forDate]="forDate">{{ngContent}}</td>`
});

export const Base = Template.bind({});
Base.args = {
	ngContent: 'convoy table cell',
	forDate: true
} as Partial<TableCellComponent>;
