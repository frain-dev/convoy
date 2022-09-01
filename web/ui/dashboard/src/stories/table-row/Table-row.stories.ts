import { Story, Meta } from '@storybook/angular/types-6-0';
import { TableRowComponent } from '../../app/components/table-row/table-row.component';
import { TableCellComponent } from '../../app/components/table-cell/table-cell.component';

export default {
	title: 'Example/TableRow',
	component: TableRowComponent,
    subcomponents: {TableCellComponent},
	argTypes: {
		class: {
			control: { type: 'text' }
		},
		forDate: {
			control: { type: 'boolean' }
		},
		active: {
			control: { type: 'boolean' }
		}
	}
} as Meta;

const Template: Story<TableRowComponent> = (args: TableRowComponent) => ({
	props: args,
	template: `<convoy-table-row class="contents" [forDate]="forDate" [active]="active">
                <convoy-table-cell class="contents" [forDate]="forDate">{{ngContent}}</convoy-table-cell>
               </convoy-table-row>`
});

export const Base = Template.bind({});
Base.args = {
	ngContent: 'convoy table row',
	forDate: true,
	active: false
} as Partial<TableRowComponent>;
